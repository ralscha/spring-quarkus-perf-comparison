package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const (
	defaultServerType   = "ccx23"
	defaultLocation     = "hel1"
	defaultImage        = "debian-12"
	defaultServerName   = "benchmark-hel1"
	defaultSSHKeyName   = "benchmark-hel1-ed25519"
	defaultSSHUser      = "root"
	defaultSSHKeyFile   = "benchmark-hel1-ed25519"
	cloudInitConfigPath = "..\\cloud-init-benchmark-host.yaml"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")

		serverName := config.Get("serverName")
		if serverName == "" {
			serverName = defaultServerName
		}

		sshKeyName := config.Get("sshKeyName")
		if sshKeyName == "" {
			sshKeyName = defaultSSHKeyName
		}

		serverType := config.Get("serverType")
		if serverType == "" {
			serverType = defaultServerType
		}

		location := config.Get("location")
		if location == "" {
			location = defaultLocation
		}

		image := config.Get("image")
		if image == "" {
			image = defaultImage
		}

		sshUser := config.Get("sshUser")
		if sshUser == "" {
			sshUser = defaultSSHUser
		}

		sshPrivateKeyPath, err := resolvePrivateKeyPath(config.Get("sshPrivateKeyPath"))
		if err != nil {
			return err
		}

		publicKey, err := ensureLocalSSHKeyPair(sshPrivateKeyPath)
		if err != nil {
			return err
		}

		userData, err := os.ReadFile(cloudInitConfigPath)
		if err != nil {
			return fmt.Errorf("read cloud-init file: %w", err)
		}

		sshKey, err := hcloud.NewSshKey(ctx, "benchmarkSshKey", &hcloud.SshKeyArgs{
			Name:      pulumi.String(sshKeyName),
			PublicKey: pulumi.String(publicKey),
			Labels: pulumi.StringMap{
				"project": pulumi.String("spring-quarkus-perf-comparison"),
				"stack":   pulumi.String(ctx.Stack()),
			},
		})
		if err != nil {
			return fmt.Errorf("create hetzner ssh key: %w", err)
		}

		server, err := hcloud.NewServer(ctx, "benchmarkHost", &hcloud.ServerArgs{
			Name:       pulumi.String(serverName),
			ServerType: pulumi.String(serverType),
			Location:   pulumi.String(location),
			Image:      pulumi.String(image),
			UserData:   pulumi.String(string(userData)),
			SshKeys: pulumi.StringArray{
				sshKey.Name,
			},
			PublicNets: hcloud.ServerPublicNetArray{
				&hcloud.ServerPublicNetArgs{
					Ipv4Enabled: pulumi.Bool(true),
					Ipv6Enabled: pulumi.Bool(true),
				},
			},
			Labels: pulumi.StringMap{
				"project": pulumi.String("spring-quarkus-perf-comparison"),
				"stack":   pulumi.String(ctx.Stack()),
				"role":    pulumi.String("benchmark-host"),
			},
		})
		if err != nil {
			return fmt.Errorf("create hetzner server: %w", err)
		}

		ctx.Export("serverName", server.Name)
		ctx.Export("serverType", server.ServerType)
		ctx.Export("location", server.Location)
		ctx.Export("image", server.Image)
		ctx.Export("ipv4Address", server.Ipv4Address)
		ctx.Export("ipv6Address", server.Ipv6Address)
		ctx.Export("sshUser", pulumi.String(sshUser))
		ctx.Export("sshPrivateKeyPath", pulumi.String(sshPrivateKeyPath))
		ctx.Export("sshPublicKey", pulumi.String(publicKey))
		ctx.Export("sshCommand", pulumi.Sprintf("ssh -i %s %s@%s", sshPrivateKeyPath, sshUser, server.Ipv4Address))

		return nil
	})
}

func resolvePrivateKeyPath(configuredPath string) (string, error) {
	if configuredPath != "" {
		return filepath.Abs(configuredPath)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".ssh", defaultSSHKeyFile), nil
}

func ensureLocalSSHKeyPair(privateKeyPath string) (string, error) {
	if _, err := os.Stat(privateKeyPath); err == nil {
		publicKey, publicKeyErr := readPublicKey(privateKeyPath)
		if publicKeyErr == nil {
			return publicKey, nil
		}

		if removeErr := os.Remove(privateKeyPath); removeErr != nil {
			return "", fmt.Errorf("remove incompatible ssh private key %q: %w", privateKeyPath, removeErr)
		}
		_ = os.Remove(privateKeyPath + ".pub")
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read ssh private key %q: %w", privateKeyPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0o700); err != nil {
		return "", fmt.Errorf("create ssh directory: %w", err)
	}

	cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", privateKeyPath, "-C", defaultSSHKeyName)
	cmd.Stdin = strings.NewReader("y\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("generate ed25519 ssh key with ssh-keygen: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return readPublicKey(privateKeyPath)
}

func readPublicKey(privateKeyPath string) (string, error) {
	publicKeyBytes, err := os.ReadFile(privateKeyPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("read ssh public key %q: %w", privateKeyPath+".pub", err)
	}

	publicKey := strings.TrimSpace(string(publicKeyBytes))
	if publicKey == "" {
		return "", fmt.Errorf("ssh public key %q is empty", privateKeyPath+".pub")
	}

	return publicKey, nil
}
