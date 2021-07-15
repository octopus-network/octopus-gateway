# octopus-gateway

gcloud builds submit --config=cloudbuild.yaml \
    --substitutions=SHORT_SHA="0.0.1",REPO_NAME="docker-repository"
