gcloud run deploy marko \
  --source . \
  --region europe-central2 \
  --allow-unauthenticated \
  --set-secrets BLUEDOT_WEBHOOK_SECRET=bluedot-webhook-secret:latest \
  --set-secrets OBSIDIAN_CREDS=obsidian-creds:latest \
  --update-env-vars OBSIDIAN_VAULT_PATH=vault \
  --update-env-vars OBSIDIAN_TRANSCRIPTS_PATH_PREFIX=marko/transcripts \
  --project marko-506115
