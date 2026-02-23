# Deploying to Google Cloud Run

## Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed and authenticated (`gcloud auth login`)
- A GCP project with billing enabled
- Docker installed (only needed for local image builds)

## One-time setup

### 1. Set your project

```bash
gcloud config set project YOUR_PROJECT_ID
```

### 2. Enable required APIs

```bash
gcloud services enable run.googleapis.com artifactregistry.googleapis.com cloudbuild.googleapis.com
```

### 3. Store your GitHub token as a secret

The service calls the GitHub API and needs a personal access token with `public_repo` read scope.

```bash
echo -n "$GITHUB_TOKEN" | gcloud secrets create github-token --data-file=-
```

Grant Cloud Run access to the secret:

```bash
gcloud secrets add-iam-policy-binding github-token \
  --member="serviceAccount:$(gcloud projects describe $(gcloud config get project) \
    --format='value(projectNumber)')-compute@developer.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

## Deploy

Run the following command (or use `make deploy`):

```bash
gcloud run deploy expmod \
  --source . \
  --region us-central1 \
  --set-secrets GITHUB_TOKEN=github-token:latest \
  --allow-unauthenticated
```

`--source .` builds the container image via Cloud Build using the `Dockerfile` in this repo.
The service URL is printed at the end of the deployment.

## Configuration

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub personal access token (injected via Secret Manager) |
| `PORT` | Port to listen on — set automatically by Cloud Run |

## Updating the secret

To rotate the GitHub token:

```bash
echo -n "$GITHUB_TOKEN" | gcloud secrets versions add github-token --data-file=-
```

## Teardown

```bash
gcloud run services delete expmod --region us-central1
```
