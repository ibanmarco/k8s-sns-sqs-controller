package vault

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault-client-go"
	"log"
	"time"
)

func ReadData(vaultToken, vaultHost string) (string, string) {
	c, err := vault.New(
		vault.WithAddress(vaultHost),
		vault.WithRequestTimeout(15*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	if err := c.SetToken(vaultToken); err != nil {
		log.Fatal(err)
	}

	resp, err := c.Read(ctx, "/demos/aws/k8scontroller")
	if err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("%v", resp.Data["AWS_ACCESS_KEY_ID"]), fmt.Sprintf("%v", resp.Data["AWS_SECRET_ACCESS_KEY"])

}
