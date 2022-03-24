"""
# DirectRunner
python hourly_api_stats.py \
    --project $PROJECT_ID \
    --subscription projects/$PROJECT_ID/subscriptions/octopus-gateway \
    --dataset $BIGQUERY_DATASET

# DataflowRunner
python hourly_api_stats.py \
    --project $PROJECT_ID \
    --region $REGION_ID \
    --subscription projects/$PROJECT_ID/subscriptions/$PUBSUB_SUBSCRIPTION \
    --dataset $BIGQUERY_DATASET \
    --runner DataflowRunner \
    --temp_location gs://$BUCKET/octopus-gateway/temp
"""

import argparse
import json
import logging
import sys
from typing import Optional, Tuple, Dict, Union

import apache_beam as beam
from apache_beam.metrics.metric import Metrics
from apache_beam.options.pipeline_options import GoogleCloudOptions
from apache_beam.options.pipeline_options import PipelineOptions
from apache_beam.options.pipeline_options import SetupOptions
from apache_beam.options.pipeline_options import StandardOptions
from apache_beam.transforms.trigger import AfterCount
from apache_beam.transforms.trigger import AfterProcessingTime
from apache_beam.transforms.trigger import AfterWatermark
from apache_beam.transforms.trigger import AccumulationMode
from apache_beam.transforms.window import FixedWindows
from apache_beam.transforms.window import TimestampedValue
from apache_beam.utils.timestamp import Timestamp


class Method(object):
    def __init__(self, chain, project, method):
        self.chain = chain
        self.project = project
        self.method = method


class MethodCoder(beam.coders.Coder):
    def encode(self, v):
        return f"{v.chain}:{v.project}:{v.method}".encode('utf-8')

    def decode(self, e):
        v = e.decode('utf-8').split(':')
        return Method(*v)

    def is_deterministic(self):
        return True


class ParseJsonRpcFn(beam.DoFn):
  def __init__(self):
    self.num_parse_errors = Metrics.counter(self.__class__, 'num_parse_errors')

  def process(self, elem):
    try:
        data = json.loads(elem).get('jsonPayload')
        if data['msg'] in ('request', 'subscription'):
            _, chain, project = data['path'].split('/')
            output = {
                'chain': chain,
                'project': project,
                'method': data['method'],
                'timestamp': data['timestamp'],
                'duration': data['duration'],
                'length': data['length'],
                'type': data['msg']
            }
            if hasattr(data, 'id'):
                output['id'] = data['id']
            if hasattr(data, 'subscription'):
                output['subscription'] = data['subscription']
            if data['error']:
                output['error'] = data['error']['code']
            yield output
    except:
        self.num_parse_errors.inc()
        logging.error('Parse error on "%s"', elem)


@beam.typehints.with_output_types(Tuple[Method, Tuple[str, float, int, Optional[int]]])
def GetMethodFn(elem):
    method = Method(elem['chain'], elem['project'], elem['method'])
    others = (elem['type'], elem['duration'], elem['length'], elem.get('error'))
    return method, others


@beam.typehints.with_input_types(Tuple[str, float, int, Optional[int]])
@beam.typehints.with_output_types(Tuple[int, int, int, float, int])
class CombineMethodFn(beam.CombineFn):
    def create_accumulator(self):
        category = 1 # 1-req 2-sub
        count = 0
        errors = 0
        duration = .0
        length = 0
        accumulator = category, count, errors, duration, length
        return accumulator

    def add_input(self, accumulator, input):
        category, count, errors, duration, length = accumulator
        t, d, l, e = input
        category = (1 if t == 'request' else 2)
        errors += (1 if e != None else 0)
        return category, count+1, errors, duration+d, length+l

    def merge_accumulators(self, accumulators):
        category, count, errors, duration, length = zip(*accumulators)
        return category[0], sum(count), sum(errors), sum(duration), sum(length)

    def extract_output(self, accumulator):
        category, count, errors, duration, length = accumulator
        # mean_duration = float('NaN')
        # if category == 1:
        #     mean_duration = (float('NaN') if count == 0 else duration/count)
        return category, count, errors, duration, length


@beam.typehints.with_input_types(Tuple[Method, Tuple[int, int, int, float, int]])
@beam.typehints.with_output_types(Dict[str, Union[str, int, float]])
class ConvertToDict(beam.DoFn):
    def process(self, elem, window=beam.DoFn.WindowParam):
        ts_format = '%Y-%m-%d %H:%M:%S.%f UTC'
        method, stats = elem
        yield {
            'chain': method.chain,
            'project': method.project,
            'method': method.method,
            'category': stats[0],
            'count': stats[1],
            'errors': stats[2],
            'duration': stats[3],
            'length': stats[4],
            'window_start': window.start.to_utc_datetime().strftime(ts_format),
            'processing_time': Timestamp.now().to_utc_datetime().strftime(ts_format)
        }


class CalculateMethodStats(beam.PTransform):
    def __init__(self, window_duration, allowed_lateness):
        self.window_duration = window_duration * 60
        self.allowed_lateness_seconds = allowed_lateness * 60

    def expand(self, pcoll):
        return (
            pcoll
            | 'FixedWindows' >> beam.WindowInto(FixedWindows(self.window_duration),
                trigger=AfterWatermark(early=AfterProcessingTime(10), late=AfterCount(1)),
                accumulation_mode=AccumulationMode.ACCUMULATING,
                allowed_lateness=self.allowed_lateness_seconds)
            | 'GetMethod' >> beam.Map(GetMethodFn)
            | 'CombineMethod' >> beam.CombinePerKey(CombineMethodFn())
        )


class WriteToBigQuery(beam.PTransform):
    def __init__(self, table_name, dataset, schema, project):
        self.table_name = table_name
        self.dataset = dataset
        self.schema = schema
        self.project = project

    def get_schema(self):
        return ', '.join('%s:%s' % (col, self.schema[col]) for col in self.schema)

    def expand(self, pcoll):
        return (
            pcoll
            | 'ConvertToRow' >> beam.Map(lambda elem: {col: elem[col] for col in self.schema})
            | beam.io.WriteToBigQuery(self.table_name, self.dataset, self.project, self.get_schema())
        )


def run(argv=None, save_main_session=True):
    parser = argparse.ArgumentParser()

    parser.add_argument(
        '--subscription', 
        type=str, 
        required=True,
        help='Pub/Sub subscription to read from')
    parser.add_argument(
        '--dataset',
        type=str,
        required=True,
        help='BigQuery Dataset to write tables to. Must already exist.')
    parser.add_argument(
        '--table_name',
        default='stats',
        help='The BigQuery table name. Should not already exist.')
    parser.add_argument(
        '--window_duration',
        type=int,
        default=60,
        help='Numeric value of fixed window duration, in minutes')
    parser.add_argument(
        '--allowed_lateness',
        type=int,
        default=10,
        help='Numeric value of allowed data lateness, in minutes')

    args, pipeline_args = parser.parse_known_args(argv)

    options = PipelineOptions(pipeline_args)
    options.view_as(SetupOptions).save_main_session = save_main_session
    options.view_as(StandardOptions).streaming = True

    if options.view_as(GoogleCloudOptions).project is None:
        parser.print_usage()
        print(sys.argv[0] + ': error: argument --project is required')
        sys.exit(1)

    with beam.Pipeline(options=options) as p:
        beam.coders.registry.register_coder(Method, MethodCoder)

        logs = (
            p
            | 'ReadPubSub' >> beam.io.ReadFromPubSub(subscription=args.subscription)
            | 'ParseAndFilter' >> beam.ParDo(ParseJsonRpcFn())
            | 'AddTimestamp' >> beam.Map(lambda elem: TimestampedValue(elem, elem['timestamp']))
        )

        (
            logs
            | 'CalcMethodStats' >> CalculateMethodStats(args.window_duration, args.allowed_lateness)
            | 'ConvertToDict' >> beam.ParDo(ConvertToDict())
            | 'WriteMethodStats' >> WriteToBigQuery(
                args.table_name + '_method_hourly',
                args.dataset,
                {
                    'chain': 'STRING',
                    'project': 'STRING',
                    'method': 'STRING',
                    'category': 'INTEGER',
                    'count': 'INTEGER',
                    'errors': 'INTEGER',
                    'duration': 'FLOAT',
                    'length': 'INTEGER',
                    'window_start': 'TIMESTAMP',
                    'processing_time': 'TIMESTAMP'
                },
                options.view_as(GoogleCloudOptions).project)
        )


if __name__ == '__main__':
    logging.getLogger().setLevel(logging.INFO)
    run()
