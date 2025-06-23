#!/bin/bash

# FHIR Validation Proxy - Cloud Run Deployment Script
# This script deploys the FHIR validation proxy to Google Cloud Run

set -e

# Configuration
PROJECT_ID=${GOOGLE_CLOUD_PROJECT:-"your-project-id"}
REGION=${GOOGLE_CLOUD_REGION:-"us-central1"}
SERVICE_NAME="fhir-validation-proxy"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 FHIR Validation Proxy - Cloud Run Deployment${NC}"
echo "=================================================="

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ gcloud CLI is not installed. Please install it first.${NC}"
    exit 1
fi

# Check if user is authenticated
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" | grep -q .; then
    echo -e "${YELLOW}⚠️  Not authenticated with gcloud. Please run: gcloud auth login${NC}"
    exit 1
fi

# Set project
echo -e "${GREEN}📋 Setting project to: ${PROJECT_ID}${NC}"
gcloud config set project ${PROJECT_ID}

# Enable required APIs
echo -e "${GREEN}🔧 Enabling required APIs...${NC}"
gcloud services enable run.googleapis.com
gcloud services enable cloudbuild.googleapis.com
gcloud services enable healthcare.googleapis.com
gcloud services enable iam.googleapis.com

# Build and push Docker image
echo -e "${GREEN}🐳 Building and pushing Docker image...${NC}"
gcloud builds submit --tag ${IMAGE_NAME} .

# Create service account if it doesn't exist
SERVICE_ACCOUNT="${SERVICE_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe ${SERVICE_ACCOUNT} &> /dev/null; then
    echo -e "${GREEN}👤 Creating service account: ${SERVICE_ACCOUNT}${NC}"
    gcloud iam service-accounts create ${SERVICE_NAME} \
        --display-name="FHIR Validation Proxy Service Account"
fi

# Grant necessary permissions
echo -e "${GREEN}🔐 Granting permissions to service account...${NC}"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/healthcare.fhirResourceEditor"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/logging.logWriter"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/monitoring.metricWriter"

# Deploy to Cloud Run
echo -e "${GREEN}🚀 Deploying to Cloud Run...${NC}"
gcloud run deploy ${SERVICE_NAME} \
    --image ${IMAGE_NAME} \
    --platform managed \
    --region ${REGION} \
    --allow-unauthenticated \
    --service-account ${SERVICE_ACCOUNT} \
    --memory 4Gi \
    --cpu 2 \
    --max-instances 100 \
    --min-instances 1 \
    --concurrency 80 \
    --timeout 300 \
    --set-env-vars="GOOGLE_CLOUD_PROJECT=${PROJECT_ID}" \
    --set-env-vars="REQUIRE_AUTHENTICATION=true" \
    --set-env-vars="AUDIT_LOGGING=true" \
    --set-env-vars="ENABLE_METRICS=true" \
    --set-env-vars="SERVER_PORT=8080" \
    --set-env-vars="SERVER_READ_TIMEOUT=30s" \
    --set-env-vars="SERVER_WRITE_TIMEOUT=30s" \
    --set-env-vars="SERVER_IDLE_TIMEOUT=60s"

# Get the service URL
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region ${REGION} --format="value(status.url)")

echo -e "${GREEN}✅ Deployment successful!${NC}"
echo "=================================================="
echo -e "${GREEN}🌐 Service URL: ${SERVICE_URL}${NC}"
echo -e "${GREEN}📊 Health Check: ${SERVICE_URL}/health${NC}"
echo -e "${GREEN}📈 Metrics: ${SERVICE_URL}/metrics${NC}"
echo -e "${GREEN}🔍 Validation: ${SERVICE_URL}/validate${NC}"
echo -e "${GREEN}🏥 FHIR Endpoint: ${SERVICE_URL}/fhir${NC}"
echo ""
echo -e "${YELLOW}📝 Next steps:${NC}"
echo "1. Update your FHIR store configuration in the environment variables"
echo "2. Configure authentication if needed"
echo "3. Set up monitoring and alerting"
echo "4. Test the endpoints"
echo ""
echo -e "${GREEN}🎉 Your FHIR validation proxy is ready for enterprise use!${NC}" 