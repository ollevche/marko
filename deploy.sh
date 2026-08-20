gcloud run deploy marko \
  --source . \
  --region europe-central2 \
  --allow-unauthenticated \
  --set-secrets BLUEDOT_WEBHOOK_SECRET=bluedot-webhook-secret:latest \
  --project marko-506115
