const { logger } = require('../../../lib/log')

var kafka = require('kafka-node'),
    Producer = kafka.Producer,
    KeyedMessage = kafka.KeyedMessage,
    client = new kafka.KafkaClient({
        kafkaHost: process.env.KAFKA_HOSTS || config.kafka.hosts,
        sasl: {
            mechanism: process.env.KAFKA_SASL_MECHANISM || config.kafka.sasl.mechanism,
            username: process.env.KAFKA_SASL_USERNAME || config.kafka.sasl.username,
            password: process.env.KAFKA_SASL_PASSWORD || config.kafka.sasl.password,
        },
        sslOptions: {
            rejectUnauthorized: true
        }
    }),
    producer = new Producer(client, {
        partitionerType: 1
    })

producer.on('ready', function () {
    logger.info('kafka ready')
})

producer.on('error', function (err) {
    logger.error('kafka error', err)
})

module.exports = {
    stat: (info) => {
        let km = new KeyedMessage(info.key, JSON.stringify(info.message))
        payloads = [
            { topic: process.env.KAFKA_TOPIC || config.kafka.topic, messages: [km] }
        ];

        producer.send(payloads, function (err, data) {
            if(err) {
                logger.error(err, data)
            }
        });
    }
} 
