#!/bin/bash

# Configuration
SERVICE_NAME="david-cloud-run"
REGION="asia-east1"
PROJECT_ID=$(gcloud config get-value project)

echo "--------------------------------------------------------"
echo "Deploying $SERVICE_NAME to Cloud Run"
echo "Project: $PROJECT_ID"
echo "Region:  $REGION"
echo "--------------------------------------------------------"

# Deploy to Cloud Run
# This command builds the source using the local Dockerfile, 
# pushes to Artifact Registry, and deploys to Cloud Run.
gcloud run deploy "$SERVICE_NAME" \
  --source . \
  --region "$REGION" \
  --set-secrets="CALDAV_USERNAME=CALDAV_USERNAME:latest,CALDAV_PASSWORD=CALDAV_PASSWORD:latest" \
  --allow-unauthenticated \
  --quiet

if [ $? -eq 0 ]; then
  echo "--------------------------------------------------------"
  echo "Deployment Successful!"
  SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format="value(status.url)")
  echo "Service URL: $SERVICE_URL"
  echo "--------------------------------------------------------"
else
  echo "--------------------------------------------------------"
  echo "Deployment Failed. Check logs for details."
  echo "--------------------------------------------------------"
  exit 1
fi
