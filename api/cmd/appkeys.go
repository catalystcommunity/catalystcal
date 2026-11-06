package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"

	"github.com/catalystcommunity/catalystcal/api/internal/actas"
)

const appKeysUsage = `catalystcal app-keys: catalystcal's LinkKeys application keys (act-as)

Usage:
  catalystcal app-keys generate --file PATH --domain DOMAIN --user-id UUID --instance-id NAME [--application-id catalystcal]
  catalystcal app-keys enroll   --file PATH --tcp-addr HOST:PORT --fingerprints FP[,FP] --api-key-file PATH
  catalystcal app-keys renew    --file PATH --tcp-addr HOST:PORT --fingerprints FP[,FP]

generate writes a new key file (mode 0600) with three Ed25519 signing keys.
It does not replace a file. The file is CATALYSTCAL_ACT_AS_APP_KEYS_FILE.

enroll does the initial enrollment at the LinkKeys server of DOMAIN. The
API key belongs to the account UUID; the home domain takes the subject
from the key. Read it from a file, never from the command line.

renew renews the attestations. The server does this every 6 hours.`

func runAppKeys(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(appKeysUsage)
		return nil
	}
	action := args[0]
	fs := flag.NewFlagSet("app-keys "+action, flag.ContinueOnError)
	file := fs.String("file", "", "key file")
	domain := fs.String("domain", "", "LinkKeys domain of the account")
	userID := fs.String("user-id", "", "LinkKeys account UUID")
	appID := fs.String("application-id", "catalystcal", "application id")
	instanceID := fs.String("instance-id", "", "instance id")
	addr := fs.String("tcp-addr", "", "LinkKeys TCP address of the home domain")
	fps := fs.String("fingerprints", "", "TLS key fingerprints of that server, comma-separated")
	keyFile := fs.String("api-key-file", "", "file with the account API key")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("--file is required")
	}
	if action == "generate" {
		k, err := actas.GenerateAppKeys(*domain, *userID, *appID, *instanceID)
		if err != nil {
			return err
		}
		if err := actas.WriteAppKeys(*file, k); err != nil {
			return err
		}
		fmt.Printf("wrote %s: %s@%s/%s instance %s, %d signing keys\n", *file, k.UserID, k.Ref().SubjectDomain, k.ApplicationID, k.InstanceID, len(k.SigningKeys))
		return nil
	}
	if action != "enroll" && action != "renew" {
		return fmt.Errorf("unknown app-keys action %q\n\n%s", action, appKeysUsage)
	}
	k, err := actas.LoadAppKeys(*file)
	if err != nil {
		return err
	}
	if *addr == "" || *fps == "" {
		return errors.New("--tcp-addr and --fingerprints are required")
	}
	credential := "application-signature"
	if action == "enroll" {
		if *keyFile == "" {
			return errors.New("--api-key-file is required for enroll")
		}
		b, err := os.ReadFile(*keyFile)
		if err != nil {
			return fmt.Errorf("read the API key: %w", err)
		}
		if credential = strings.TrimSpace(string(b)); credential == "" {
			return errors.New("the API key file is empty")
		}
	}
	t, err := regularrp.NewPinnedRpcTransport(regularrp.PinnedRpcTransportOptions{
		TCPAddress: *addr, Fingerprints: strings.Split(*fps, ","), APIKey: credential,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if action == "enroll" {
		if err := actas.Enroll(ctx, t, k); err != nil {
			return err
		}
		fmt.Printf("enrolled %s@%s/%s instance %s\n", k.UserID, k.Ref().SubjectDomain, k.ApplicationID, k.InstanceID)
		return nil
	}
	if err := actas.RenewAttestations(ctx, t, k); err != nil {
		return err
	}
	fmt.Printf("renewed %d attestations\n", len(k.SigningKeys))
	return nil
}
