package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func (c *Client) RemoteManifestDigest(ctx context.Context, ref string) (string, error) {
	authCfg, ok, err := registryAuthConfig(ref)
	if err != nil {
		return "", err
	}
	parsed, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return "", fmt.Errorf("parse reference %q: %w", ref, err)
	}
	auth := registryAuthenticator(authCfg, ok)
	desc, err := remote.Head(parsed, remote.WithContext(ctx), remote.WithAuth(auth))
	if err != nil {
		return "", annotateRegistryAuth(ref, encodedAuthForAnnotate(authCfg, ok), err)
	}
	if desc == nil {
		return "", fmt.Errorf("empty manifest descriptor for %q", ref)
	}
	return desc.Digest.String(), nil
}

func registryAuthenticator(authCfg registry.AuthConfig, ok bool) authn.Authenticator {
	if !ok {
		return authn.Anonymous
	}
	if authCfg.Username == "" && authCfg.Password == "" && authCfg.IdentityToken == "" && authCfg.RegistryToken == "" {
		return authn.Anonymous
	}
	return authn.FromConfig(authn.AuthConfig{
		Username:      authCfg.Username,
		Password:      authCfg.Password,
		IdentityToken: authCfg.IdentityToken,
		RegistryToken: authCfg.RegistryToken,
	})
}
