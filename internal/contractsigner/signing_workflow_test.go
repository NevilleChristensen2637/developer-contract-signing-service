package contractsigner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/developer-contract-signing-service/internal/infrai"
)

func TestEvaluateSigning(t *testing.T) {
	tests := []struct {
		name   string
		input  ContractSignRequest
		want   string
		mark   string
		reason string
	}{
		{
			name: "ready for approved production release",
			input: ContractSignRequest{
				ContractID:     "devtools-cli-2026-04",
				VendorName:     "Acme Signing Co",
				ProductName:    "acmectl",
				ReleaseVersion: "2.4.1",
				Build: BuildEvent{CommitSHA: "8f3c2ab1", Status: "passed", ArtifactName: "acmectl_linux_amd64.tar.gz"},
				Release: ReleaseOperation{Environment: "production", ChangeTicket: "REL-2048", Approved: true},
			},
			want:   "ready",
			mark:   "RELEASE APPROVED",
			reason: "Production release approved after a passing build.",
		},
		{
			name: "review required when approval missing",
			input: ContractSignRequest{
				ContractID:     "devtools-cli-2026-04",
				VendorName:     "Acme Signing Co",
				ProductName:    "acmectl",
				ReleaseVersion: "2.4.1",
				Build: BuildEvent{CommitSHA: "8f3c2ab1", Status: "passed", ArtifactName: "acmectl_linux_amd64.tar.gz"},
				Release: ReleaseOperation{Environment: "production", ChangeTicket: "REL-2048", Approved: false},
			},
			want:   "review_required",
			mark:   "REVIEW REQUIRED",
			reason: "Contract needs manual release review before signing.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateSigning(tt.input)
			if got.SigningStatus != tt.want {
				t.Fatalf("status = %q, want %q", got.SigningStatus, tt.want)
			}
			if got.WatermarkText != tt.mark {
				t.Fatalf("watermark = %q, want %q", got.WatermarkText, tt.mark)
			}
			if got.Reason != tt.reason {
				t.Fatalf("reason = %q, want %q", got.Reason, tt.reason)
			}
		})
	}
}

func TestPrepareContractUsesExpectedPDFBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/v1/pdf/generate":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization = %q", got)
			}
			if !strings.Contains(string(body), "\"page_size\":\"A4\"") {
				t.Fatalf("generate request missing page_size: %s", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"data":{"pdf":"Z2VuZXJhdGVkLXBkZg=="},"error":null,"metadata":{}}`))
		case "/v1/pdf/watermark":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if !strings.Contains(string(body), "RELEASE APPROVED") {
				t.Fatalf("watermark request missing decision text: %s", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"data":{"pdf":"d2F0ZXJtYXJrZWQtcGRm"},"error":null,"metadata":{}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := infrai.NewClient("test-key", server.Client())
	setTestBaseURL(client, server.URL+"/v1")
	service := NewService(client)

	resp, err := service.PrepareContract(context.Background(), ContractSignRequest{
		ContractID:     "devtools-cli-2026-04",
		VendorName:     "Acme Signing Co",
		ProductName:    "acmectl",
		ReleaseVersion: "2.4.1",
		Build:          BuildEvent{CommitSHA: "8f3c2ab1", Status: "passed", ArtifactName: "acmectl_linux_amd64.tar.gz"},
		Release:        ReleaseOperation{Environment: "production", ChangeTicket: "REL-2048", Approved: true},
		Certificate:    CertificateMaterial{CertPEM: "cert", KeyPEM: "key"},
	})
	if err != nil {
		t.Fatalf("PrepareContract error = %v", err)
	}
	if resp.SigningStatus != "ready" {
		t.Fatalf("status = %q, want ready", resp.SigningStatus)
	}
	if resp.PDFBase64 != "d2F0ZXJtYXJrZWQtcGRm" {
		t.Fatalf("pdf = %q", resp.PDFBase64)
	}
}

func TestPrepareContractRejectsEmptyGeneratedPDF(t *testing.T) {
	watermarkCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pdf/generate":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"data":{"pdf":"  "}}`))
		case "/v1/pdf/watermark":
			watermarkCalled = true
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := infrai.NewClient("test-key", server.Client())
	setTestBaseURL(client, server.URL+"/v1")
	_, err := NewService(client).PrepareContract(context.Background(), ContractSignRequest{})
	if err == nil || !strings.Contains(err.Error(), "pdf.generate returned an empty PDF") {
		t.Fatalf("error = %v, want empty PDF error", err)
	}
	if watermarkCalled {
		t.Fatal("watermark called with empty PDF")
	}
}

func setTestBaseURL(client *infrai.Client, baseURL string) {
	raw, _ := json.Marshal(struct{}{})
	_ = raw
	clientReflectSetBaseURL(client, baseURL)
}
