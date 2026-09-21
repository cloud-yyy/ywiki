// Package config loads, saves, and resolves ywiki authentication settings.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

// OrgType identifies the kind of organization, which selects the header the
// organization ID travels in: Yandex 360 for Business uses X-Org-Id, while
// Yandex Cloud and Yandex Identity Hub organizations use X-Cloud-Org-Id.
type OrgType string

const (
	// OrgType360 is a Yandex 360 for Business organization.
	OrgType360 OrgType = "360"

	// OrgTypeCloud is a Yandex Cloud or Yandex Identity Hub organization.
	OrgTypeCloud OrgType = "cloud"
)

// OrgHeader returns the header that carries the organization ID.
func (t OrgType) OrgHeader() string {
	if t == OrgTypeCloud {
		return "X-Cloud-Org-Id"
	}
	return "X-Org-Id"
}

// DefaultTokenType is the token type assumed when none is configured. A 360
// organization only accepts OAuth; a Cloud organization defaults to IAM,
// which is what ywiki sent before token type was configurable, so configs
// written by earlier versions keep working unchanged.
func (t OrgType) DefaultTokenType() TokenType {
	if t == OrgTypeCloud {
		return TokenTypeIAM
	}
	return TokenTypeOAuth
}

// TokenType identifies the kind of token, which selects the Authorization
// scheme. It is independent of the organization type: Identity Hub pairs an
// OAuth token with the Cloud organization header.
type TokenType string

const (
	// TokenTypeOAuth is a Yandex OAuth token, sent as "OAuth <token>".
	TokenTypeOAuth TokenType = "oauth"

	// TokenTypeIAM is a Yandex Cloud IAM token, sent as "Bearer <token>".
	TokenTypeIAM TokenType = "iam"
)

// AuthScheme returns the Authorization header scheme for the token type.
func (t TokenType) AuthScheme() string {
	if t == TokenTypeIAM {
		return "Bearer"
	}
	return "OAuth"
}

// Config holds the persistent ywiki configuration loaded from config.yaml.
// The flat structure (no profiles) is intentional for v1 simplicity.
type Config struct {
	// Token is the OAuth or IAM token used to authenticate API requests.
	Token string `yaml:"token,omitempty"`

	// OrgID is the Wiki organization ID.
	OrgID string `yaml:"org_id,omitempty"`

	// OrgType selects the organization header.
	OrgType OrgType `yaml:"org_type,omitempty"`

	// TokenType selects the Authorization scheme. Empty means the
	// organization type's default.
	TokenType TokenType `yaml:"token_type,omitempty"`
}

// AuthFlags carries credential values given on the command line.
type AuthFlags struct {
	Token     string
	OrgID     string
	OrgType   string
	TokenType string
}

// ResolvedAuth holds resolved credentials together with the tier they came from.
type ResolvedAuth struct {
	// Token is the resolved OAuth or IAM token.
	Token string

	// OrgID is the resolved organization ID.
	OrgID string

	// OrgType selects the organization header.
	OrgType OrgType

	// TokenType selects the Authorization scheme.
	TokenType TokenType

	// TokenSource indicates where credentials came from: "flag", "env", or "config".
	TokenSource string
}

const (
	dirPermissions  = 0o700 // Owner read/write/execute for the config directory.
	filePermissions = 0o600 // Owner read/write for the config file.
)

// Load reads the config file and returns the parsed Config.
// A missing config file yields an empty Config and no error.
func Load() (*Config, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine config path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes the config to disk atomically with secure permissions.
// The directory is created (and forced to 0700) if needed, and the file is
// written to a temp file first so an interrupted write cannot corrupt it.
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to determine config directory: %w", err)
	}

	if mkdirErr := os.MkdirAll(dir, dirPermissions); mkdirErr != nil {
		return fmt.Errorf("failed to create config directory: %w", mkdirErr)
	}

	// MkdirAll leaves an existing directory's permissions untouched, so a
	// pre-existing config dir could keep looser permissions on the token
	// store. Enforce 0700 on the leaf.
	if chmodErr := os.Chmod(dir, dirPermissions); chmodErr != nil {
		return fmt.Errorf("failed to secure config directory: %w", chmodErr)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path, err := ConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}

	tmpPath := path + ".tmp"

	// Remove any stale temp file so the new one is created fresh with 0600:
	// os.WriteFile does not re-apply permissions to an existing file, which
	// could leave the token in a world-readable file. O_EXCL then guarantees
	// we create it ourselves (no clobber, no symlink follow).
	_ = os.Remove(tmpPath)
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePermissions)
	if err != nil {
		return fmt.Errorf("failed to create config temp file: %w", err)
	}
	if _, writeErr := f.Write(data); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write config: %w", writeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write config: %w", closeErr)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// Delete removes the config file. A missing file is not an error.
func Delete() error {
	path, err := ConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config: %w", err)
	}

	return nil
}

// ParseOrgType validates and normalizes an organization type.
func ParseOrgType(value string) (OrgType, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case string(OrgType360):
		return OrgType360, nil
	case string(OrgTypeCloud):
		return OrgTypeCloud, nil
	default:
		return "", fmt.Errorf("invalid org-type %q", value)
	}
}

// ParseTokenType validates and normalizes a token type. An empty value is
// valid and means "use the organization type's default".
func ParseTokenType(value string) (TokenType, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "":
		return "", nil
	case string(TokenTypeOAuth):
		return TokenTypeOAuth, nil
	case string(TokenTypeIAM):
		return TokenTypeIAM, nil
	default:
		return "", fmt.Errorf("invalid token-type %q", value)
	}
}

// ResolveAuth resolves credentials using three-tier precedence:
// flags > environment variables > config file. Flag and config tiers are
// strict: partial credentials are reported rather than silently skipped, so a
// typo does not fall through to a different tier. The environment tier is only
// used when complete. Token type is optional in every tier and is taken from
// the same tier as the rest of the credentials.
func ResolveAuth(flags AuthFlags) (*ResolvedAuth, error) {
	flagToken, flagOrgID, flagOrgType := flags.Token, flags.OrgID, flags.OrgType

	// Tier 1: flags.
	if hasCompleteAuth(flagToken, flagOrgID, flagOrgType) {
		return resolveAuthTier("flag", "", flagToken, flagOrgID, flagOrgType, flags.TokenType)
	}
	if hasAnyAuth(flagToken, flagOrgID, flagOrgType) {
		if flagOrgType != "" {
			if _, err := ParseOrgType(flagOrgType); err != nil {
				return nil, wikierrors.NewUserError(err.Error(), invalidOrgTypeSuggestion("flag", ""))
			}
		}
		return nil, wikierrors.NewUserError(
			"incomplete flag authentication",
			incompleteAuthSuggestion("flag", ""),
		)
	}

	// Tier 2: environment variables.
	envToken := os.Getenv("YWIKI_TOKEN")
	envOrgID := os.Getenv("YWIKI_ORG_ID")
	envOrgType := os.Getenv("YWIKI_ORG_TYPE")
	if hasCompleteAuth(envToken, envOrgID, envOrgType) {
		return resolveAuthTier(
			"env", "", envToken, envOrgID, envOrgType, os.Getenv("YWIKI_TOKEN_TYPE"),
		)
	}

	// Tier 3: config file.
	cfgPath, err := ConfigFilePath()
	if err != nil {
		return nil, wikierrors.NewUserError(
			fmt.Sprintf("failed to determine config path: %v", err),
			"Set YWIKI_CONFIG_DIR or provide credentials via flags or environment variables",
		)
	}

	cfg, err := Load()
	if err != nil {
		return nil, wikierrors.NewUserError(
			fmt.Sprintf("failed to load config: %v", err),
			fmt.Sprintf(
				"Fix or remove %s, or provide credentials via flags or environment variables",
				cfgPath,
			),
		)
	}
	if hasCompleteAuth(cfg.Token, cfg.OrgID, string(cfg.OrgType)) {
		return resolveAuthTier(
			"config", cfgPath, cfg.Token, cfg.OrgID, string(cfg.OrgType), string(cfg.TokenType),
		)
	}
	if hasAnyAuth(cfg.Token, cfg.OrgID, string(cfg.OrgType)) {
		if cfg.OrgType != "" {
			if _, err := ParseOrgType(string(cfg.OrgType)); err != nil {
				return nil, wikierrors.NewUserError(
					err.Error(),
					invalidOrgTypeSuggestion("config", cfgPath),
				)
			}
		}
		return nil, wikierrors.NewUserError(
			"incomplete config authentication",
			incompleteAuthSuggestion("config", cfgPath),
		)
	}

	return nil, wikierrors.NewAuthError(
		"not authenticated",
		"Run: ywiki auth login\n"+
			"Or set: export YWIKI_TOKEN=<token> YWIKI_ORG_ID=<org> YWIKI_ORG_TYPE=<360|cloud>",
	)
}

func hasAnyAuth(token, orgID, orgType string) bool {
	return token != "" || orgID != "" || orgType != ""
}

func hasCompleteAuth(token, orgID, orgType string) bool {
	return token != "" && orgID != "" && orgType != ""
}

func resolveAuthTier(
	source, configPath, token, orgID, orgTypeRaw, tokenTypeRaw string,
) (*ResolvedAuth, error) {
	orgType, err := ParseOrgType(orgTypeRaw)
	if err != nil {
		return nil, wikierrors.NewUserError(
			err.Error(),
			invalidOrgTypeSuggestion(source, configPath),
		)
	}

	tokenType, err := ParseTokenType(tokenTypeRaw)
	if err != nil {
		return nil, wikierrors.NewUserError(
			err.Error(),
			"Use token type oauth or iam",
		)
	}
	if tokenType == "" {
		tokenType = orgType.DefaultTokenType()
	}

	return &ResolvedAuth{
		Token:       token,
		OrgID:       orgID,
		OrgType:     orgType,
		TokenType:   tokenType,
		TokenSource: source,
	}, nil
}

func invalidOrgTypeSuggestion(source, configPath string) string {
	switch source {
	case "flag":
		return "Use --org-type 360 or --org-type cloud"
	case "env":
		return "Set YWIKI_ORG_TYPE to 360 or cloud"
	case "config":
		return fmt.Sprintf("Set org_type to 360 or cloud in %s, or run ywiki auth login", configPath)
	default:
		return "Use org-type 360 or cloud"
	}
}

func incompleteAuthSuggestion(source, configPath string) string {
	switch source {
	case "flag":
		return "Provide --token, --org-id, and --org-type together"
	case "config":
		return fmt.Sprintf("Set token, org_id, and org_type in %s, or run ywiki auth login", configPath)
	default:
		return "Provide token, org_id, and org_type together"
	}
}
