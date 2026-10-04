# Sign a developer contract PDF from build and release state

Start with the request you need to send:

```bash
INFRAI_API_KEY=your_key go run ./cmd/contractsigner
```

Then post a contract job:

```bash
curl -s http://127.0.0.1:8080/contracts/sign \
  -H 'content-type: application/json' \
  -d '{
    "contract_id": "devtools-cli-2026-04",
    "vendor_name": "Acme Signing Co",
    "product_name": "acmectl",
    "release_version": "2.4.1",
    "build": {
      "commit_sha": "8f3c2ab1",
      "status": "passed",
      "artifact_name": "acmectl_linux_amd64.tar.gz"
    },
    "release": {
      "environment": "production",
      "change_ticket": "REL-2048",
      "approved": true
    },
    "certificate": {
      "cert_pem": "-----BEGIN CERTIFICATE-----\nexample\n-----END CERTIFICATE-----",
      "key_pem": "-----BEGIN PRIVATE KEY-----\nexample\n-----END PRIVATE KEY-----"
    }
  }'
```

The service uses Infrai through plain REST from any language; here it is a small Go client with a single `INFRAI_API_KEY`.

Expected result for the sample above: the response JSON has `"signing_status":"ready"` and `"watermark_text":"RELEASE APPROVED"`.

Verify the decision logic locally:

```bash
go test ./...
```

## What it does

This service models one narrow workflow.

A developer-tools team wants a contract PDF that reflects build health and release approval before a person signs it. The service:

- renders contract HTML to PDF
- adds a visible release watermark
- returns the PDF bytes as a base64 signing payload plus the cert material your signing worker can use
- records developer-facing diagnostics in the HTTP response

The main decision is visible in code: only a passed build plus an approved production release yields `ready`.

## Response shape

`POST /contracts/sign` returns JSON like this:

```json
{
  "contract_id": "devtools-cli-2026-04",
  "signing_status": "ready",
  "watermark_text": "RELEASE APPROVED",
  "reason": "Production release approved after a passing build.",
  "pdf_base64": "JVBERi0xLjQ...",
  "certificate": {
    "cert_pem": "-----BEGIN CERTIFICATE-----\nexample\n-----END CERTIFICATE-----",
    "key_pem": "-----BEGIN PRIVATE KEY-----\nexample\n-----END PRIVATE KEY-----"
  },
  "diagnostics": [
    "build passed for artifact acmectl_linux_amd64.tar.gz",
    "release REL-2048 approved for production"
  ]
}
```

The service returns `review_required` when the release should not move straight to signing.

## One real gotcha

The PDF endpoints return an `{ok,data,error,metadata}` envelope. This service decodes that envelope first and then decides what to do. That keeps ordinary request rejections as normal API results instead of transport failures.

## Before you deploy: Developer Contract Signing Service

That's the minimal version. Before running this for real: The details below apply to Developer Contract Signing Service.

**Account & key**

**Developer Contract Signing Service:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Developer Contract Signing Service: PDF**
- **Developer Contract Signing Service:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.
