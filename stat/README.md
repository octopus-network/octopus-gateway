# Architecture
GKE --> Logging --> Pub/Sub --> Dataflow --> BigQuery (PostgreSQL?)


# Prepare
- Pub/Sub
```
# Create Pub/Sub topics
gcloud pubsub topics create octopus-gateway --project ${PROJECT_ID}

# Create Pub/Sub subscriptions
gcloud pubsub subscriptions create octopus-gateway --topic octopus-gateway --project ${PROJECT_ID}
```
- Cloud Logging
```
# Create Log Export sinks
gcloud logging sinks create octopus-gateway \
    pubsub.googleapis.com/projects/${PROJECT_ID}/topics/octopus-gateway \
    --log-filter="octopus-gateway-router" \
    --project=${PROJECT_ID}
```
- Cloud Storage

- Python
```
pip install apache-beam[gcp,test]
```
