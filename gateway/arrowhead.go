package main

// The Arrowhead side of the gateway. Each step is one explicit SDK call that YOU
// implement (assignment Phase 1). Nothing here runs implicitly: main calls the steps
// in order and stops at the first one that is not done yet.

import (
	"context"
	"errors"

	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/ca"
	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/identity"
	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/models"
	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/registry"
	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/transport"
)

// errTODO marks a step the assignment asks you to implement.
var errTODO = errors.New("not implemented yet (TODO)")

// ArrowheadConfig holds the endpoints and identity of the gateway system.
type ArrowheadConfig struct {
	SystemName string // PascalCase, e.g. ColdChainGateway; also the certificate name
	Password   string // set by Sysop when creating the identity
	CAPlainURL string // http://localhost:8787  (onboarding, CA info)
	CATLSURL   string // https://localhost:8788 (device and system certificates)
	SRURL      string // https://localhost:8490, TLS server name "serviceregistry"
	AuthURL    string // https://localhost:8491, TLS server name "authentication"
	CertDir    string // where certificate, key and ca.crt are kept
	AdvertAddr string // address consumers use to reach this gateway (e.g. localhost)
	AdvertPort int    // port of the gateway's HTTP server
}

// Step 1 — certificate. Obtain a system certificate for cfg.SystemName through
// profile-ca's three steps and return credentials for mutual TLS.
//
// TODO: ca.New(plain).Info; RequestCertificate(models.ProfileOnboarding, ...) over
// plain HTTP; then ProfileDevice over mTLS presenting the onboarding certificate
// (ca.Credentials + transport.NewMTLS); then ProfileSystem presenting the device
// certificate. Save the result with ca.Save so a restart can reuse it.
func obtainCertificate(ctx context.Context, cfg ArrowheadConfig) (transport.Credentials, error) {
	_ = ca.New
	_ = models.ProfileSystem
	return transport.Credentials{}, errTODO
}

// Step 2 — identity. Log in as cfg.SystemName and return the token the registry
// requires. (Sysop creates the identity first; see api-quick-reference.md.)
//
// TODO: transport.NewMTLS(cfg.AuthURL, "authentication", creds) and
// identity.New(t).Login(ctx, cfg.SystemName, cfg.Password).
func login(ctx context.Context, cfg ArrowheadConfig, creds transport.Credentials) (string, error) {
	_ = identity.New
	return "", errTODO
}

// Step 3 — registration. Register one service per sensor quantity with the three
// interface properties orchestration needs (accessAddresses, accessPort, basePath).
//
// TODO: transport.NewMTLS(cfg.SRURL, "serviceregistry", creds), then for each
// service registry.New(t).Register(ctx, token, models.ServiceRegistration{...})
// with registry.HTTPInterface(...). Service definitions are camelCase.
func registerServices(ctx context.Context, cfg ArrowheadConfig, creds transport.Credentials, token string) error {
	_ = registry.HTTPInterface
	return errTODO
}
