package artifacts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadCredentials reads S3 credentials from the standard AWS places, so the
// secret never needs to be in KeroAgile's own config:
//
//   - profile named: that profile of the shared credentials file
//     (AWS_SHARED_CREDENTIALS_FILE, or ~/.aws/credentials);
//   - otherwise AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY (and
//     AWS_SESSION_TOKEN) when set;
//   - otherwise the AWS_PROFILE profile, or "default", from the file.
//
// Errors never include the secret.
func LoadCredentials(profile string) (Credentials, error) {
	if profile == "" {
		if id, secret := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"); id != "" && secret != "" {
			return Credentials{AccessKey: id, SecretKey: secret, SessionToken: os.Getenv("AWS_SESSION_TOKEN")}, nil
		}
		profile = os.Getenv("AWS_PROFILE")
		if profile == "" {
			profile = "default"
		}
	}
	file := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if file == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Credentials{}, err
		}
		file = filepath.Join(home, ".aws", "credentials")
	}
	f, err := os.Open(file)
	if err != nil {
		return Credentials{}, fmt.Errorf("S3 credentials: profile %q: %w", profile, err)
	}
	defer f.Close()
	var c Credentials
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimPrefix(strings.Trim(line, "[]"), "profile "))
			in = name == profile
			continue
		}
		if !in {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "aws_access_key_id":
			c.AccessKey = strings.TrimSpace(v)
		case "aws_secret_access_key":
			c.SecretKey = strings.TrimSpace(v)
		case "aws_session_token":
			c.SessionToken = strings.TrimSpace(v)
		}
	}
	if err := sc.Err(); err != nil {
		return Credentials{}, err
	}
	if c.AccessKey == "" || c.SecretKey == "" {
		return Credentials{}, errors.New("S3 credentials: profile " + profile + " not found, or without aws_access_key_id and aws_secret_access_key, in " + file)
	}
	return c, nil
}
