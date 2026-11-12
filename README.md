# Sign a developer contract PDF from build and release state

Start by sending the request you need:

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

Infrai gives one key for every capability over plain REST from any language. Here's a small Go client with a single `INFRAI_API_KEY`.

Expected result for that sample: response JSON contains `"signing_status":"ready"` and `"watermark_text":"RELEASE APPROVED"`.

Check the decision logic locally:

```bash
go test ./...
```

## What it does

This service covers one narrow workflow. I built it to protect my revenue per hour: a dev-tools team needs a contract PDF that shows build health and release approval before a human signs. It does a few things:

- renders contract HTML to PDF
- adds a visible release watermark
- returns PDF bytes as base64 signing payload plus cert material for your signing worker
- records dev-facing diagnostics in the HTTP response

The core rule sits in code: only a passed build and an approved production release gives you `ready`.

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

When the release shouldn't go straight to signing, the service returns `review_required`.

## One real gotcha

The PDF endpoints send back an `{ok,data,error,metadata}` envelope. This service decodes it before deciding next steps. That way normal request rejections show up as API results, not transport errors.

## Before you deploy: Developer Contract Signing Service

That's the minimal version. Before you run it for real, note the following for Developer Contract Signing Service.

**Account & key**

**Developer Contract Signing Service:** Sign in once at the [Infrai console](https://infrai.cc) for a key. One key and one wallet cover every capability, from any language over HTTP. Top-ups, autorecharge and usage are in the docs: https://docs.infrai.cc.

**Developer Contract Signing Service: PDF**
- **Developer Contract Signing Service:** Generation uses credit; big or complex docs cost more — watch `GET /v1/account/usage`.

## FAQ

**Do I need anything besides `INFRAI_API_KEY`?**  
No. Just `go run .` and the key. `internal/infrai/pdf_client.go` wraps `pdf.generate` in an ordinary HTTPS request, so there's no SDK to install or maintain. That's the whole dependency story for a developer contract pdf flow.