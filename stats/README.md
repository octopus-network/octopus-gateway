# Architecture
GKE --> Logging --> Pub/Sub --> Dataflow --> BigQuery (PostgreSQL?)


# Prepare
- env
```
export PROJECT_ID="bigdata-329111"
```

- Pub/Sub
```
# Create Pub/Sub topics
gcloud pubsub topics create octopus-gateway --project ${PROJECT_ID}

# Create Pub/Sub subscriptions
gcloud pubsub subscriptions create octopus-gateway --topic octopus-gateway --project ${PROJECT_ID}
```

- Cloud Logging
```
# Log Filter
export LOG_FILTER='resource.type="k8s_container" AND resource.labels.project_id="bigdata-329111" AND resource.labels.location="asia-east1" AND resource.labels.cluster_name="autopilot-cluster-1" AND resource.labels.namespace_name="gateway" AND resource.labels.container_name="router" AND resource.labels.pod_name=~"^octopus-gateway-router-*"'

# Create Log Export sinks
gcloud logging sinks create octopus-gateway \
    pubsub.googleapis.com/projects/${PROJECT_ID}/topics/octopus-gateway \
    --log-filter=${LOG_FILTER} \
    --project=${PROJECT_ID}
```

- Cloud Storage

- Python
```
pip install apache-beam[gcp,test]
```
