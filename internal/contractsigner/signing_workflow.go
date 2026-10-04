package contractsigner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"

	"github.com/example/developer-contract-signing-service/internal/infrai"
)

type BuildEvent struct {
	CommitSHA    string `json:"commit_sha"`
	Status       string `json:"status"`
	ArtifactName string `json:"artifact_name"`
}

type ReleaseOperation struct {
	Environment  string `json:"environment"`
	ChangeTicket string `json:"change_ticket"`
	Approved     bool   `json:"approved"`
}

type CertificateMaterial struct {
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
}

type ContractSignRequest struct {
	ContractID    string              `json:"contract_id"`
	VendorName    string              `json:"vendor_name"`
	ProductName   string              `json:"product_name"`
	ReleaseVersion string             `json:"release_version"`
	Build         BuildEvent          `json:"build"`
	Release       ReleaseOperation    `json:"release"`
	Certificate   CertificateMaterial `json:"certificate"`
}

type ContractSignResponse struct {
	ContractID    string              `json:"contract_id"`
	SigningStatus string              `json:"signing_status"`
	WatermarkText string              `json:"watermark_text"`
	Reason        string              `json:"reason"`
	PDFBase64     string              `json:"pdf_base64"`
	Certificate   CertificateMaterial `json:"certificate"`
	Diagnostics   []string            `json:"diagnostics"`
}

type Decision struct {
	SigningStatus string
	WatermarkText string
	Reason        string
	Diagnostics   []string
}

type PDFAPI interface {
	PDF() infrai.PDFService
}

type Service struct {
	client PDFAPI
}

func NewService(client PDFAPI) *Service {
	return &Service{client: client}
}

func EvaluateSigning(req ContractSignRequest) Decision {
	diagnostics := []string{}
	if req.Build.Status == "passed" {
		diagnostics = append(diagnostics, fmt.Sprintf("build passed for artifact %s", req.Build.ArtifactName))
	} else {
		diagnostics = append(diagnostics, fmt.Sprintf("build status is %s for artifact %s", req.Build.Status, req.Build.ArtifactName))
	}

	if req.Release.Approved {
		diagnostics = append(diagnostics, fmt.Sprintf("release %s approved for %s", req.Release.ChangeTicket, req.Release.Environment))
	} else {
		diagnostics = append(diagnostics, fmt.Sprintf("release %s pending approval for %s", req.Release.ChangeTicket, req.Release.Environment))
	}

	if req.Build.Status == "passed" && req.Release.Environment == "production" && req.Release.Approved {
		return Decision{
			SigningStatus: "ready",
			WatermarkText: "RELEASE APPROVED",
			Reason:        "Production release approved after a passing build.",
			Diagnostics:   diagnostics,
		}
	}

	return Decision{
		SigningStatus: "review_required",
		WatermarkText: "REVIEW REQUIRED",
		Reason:        "Contract needs manual release review before signing.",
		Diagnostics:   diagnostics,
	}
}

func (s *Service) PrepareContract(ctx context.Context, req ContractSignRequest) (ContractSignResponse, error) {
	decision := EvaluateSigning(req)
	htmlDoc, err := renderContractHTML(req, decision)
	if err != nil {
		return ContractSignResponse{}, err
	}

	generated, err := s.client.PDF().Generate(ctx, infrai.GenerateRequest{
		HTML:        htmlDoc,
		PageSize:    "A4",
		Orientation: "portrait",
	})
	if err != nil {
		return ContractSignResponse{}, err
	}
	if strings.TrimSpace(generated.PDF) == "" {
		return ContractSignResponse{}, fmt.Errorf("pdf.generate returned an empty PDF")
	}

	watermarked, err := s.client.PDF().Watermark(ctx, infrai.WatermarkRequest{
		PDF:      generated.PDF,
		Text:     decision.WatermarkText,
		Opacity:  0.18,
		Position: "center",
	})
	if err != nil {
		return ContractSignResponse{}, err
	}

	return ContractSignResponse{
		ContractID:    req.ContractID,
		SigningStatus: decision.SigningStatus,
		WatermarkText: decision.WatermarkText,
		Reason:        decision.Reason,
		PDFBase64:     watermarked.PDF,
		Certificate:   req.Certificate,
		Diagnostics:   decision.Diagnostics,
	}, nil
}

func renderContractHTML(req ContractSignRequest, decision Decision) (string, error) {
	const contractTemplate = `<!doctype html>
<html>
  <body style="font-family: sans-serif; margin: 40px;">
    <h1>Developer Tools Contract</h1>
    <p><strong>Contract ID:</strong> {{.ContractID}}</p>
    <p><strong>Vendor:</strong> {{.VendorName}}</p>
    <p><strong>Product:</strong> {{.ProductName}}</p>
    <p><strong>Release:</strong> {{.ReleaseVersion}}</p>
    <h2>Build event</h2>
    <p>Commit {{.Build.CommitSHA}} produced {{.Build.ArtifactName}} with status {{.Build.Status}}.</p>
    <h2>Release operation</h2>
    <p>Change {{.Release.ChangeTicket}} targets {{.Release.Environment}}. Approved: {{.Release.Approved}}.</p>
    <h2>Signing decision</h2>
    <p>Status: {{.Decision.SigningStatus}}</p>
    <p>{{.Decision.Reason}}</p>
    <h2>Diagnostics</h2>
    <ul>{{range .Decision.Diagnostics}}<li>{{.}}</li>{{end}}</ul>
  </body>
</html>`

	tmpl, err := template.New("contract").Parse(contractTemplate)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	data := struct {
		ContractID     string
		VendorName     string
		ProductName    string
		ReleaseVersion string
		Build          BuildEvent
		Release        ReleaseOperation
		Decision       Decision
	}{
		ContractID:     req.ContractID,
		VendorName:     req.VendorName,
		ProductName:    req.ProductName,
		ReleaseVersion: req.ReleaseVersion,
		Build:          req.Build,
		Release:        req.Release,
		Decision:       decision,
	}
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func DecodePDF(b64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
}
