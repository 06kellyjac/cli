package googlepubsub

import (
	"fmt"
	"io"

	"github.com/fastly/cli/pkg/cmd"
	"github.com/fastly/cli/pkg/commands/compute/manifest"
	"github.com/fastly/cli/pkg/config"
	"github.com/fastly/cli/pkg/errors"
	"github.com/fastly/cli/pkg/text"
	"github.com/fastly/go-fastly/v3/fastly"
)

// ListCommand calls the Fastly API to list Google Cloud Pub/Sub logging endpoints.
type ListCommand struct {
	cmd.Base
	manifest       manifest.Data
	Input          fastly.ListPubsubsInput
	serviceVersion cmd.OptionalServiceVersion
}

// NewListCommand returns a usable command registered under the parent.
func NewListCommand(parent cmd.Registerer, globals *config.Data) *ListCommand {
	var c ListCommand
	c.Globals = globals
	c.manifest.File.SetOutput(c.Globals.Output)
	c.manifest.File.Read(manifest.Filename)
	c.CmdClause = parent.Command("list", "List Google Cloud Pub/Sub endpoints on a Fastly service version")
	c.RegisterServiceIDFlag(&c.manifest.Flag.ServiceID)
	c.RegisterServiceVersionFlag(cmd.ServiceVersionFlagOpts{
		Dst: &c.serviceVersion.Value,
	})
	return &c
}

// Exec invokes the application logic for the command.
func (c *ListCommand) Exec(in io.Reader, out io.Writer) error {
	serviceID, serviceVersion, err := cmd.ServiceDetails(cmd.ServiceDetailsOpts{
		AllowActiveLocked:  true,
		Client:             c.Globals.Client,
		Manifest:           c.manifest,
		Out:                out,
		ServiceVersionFlag: c.serviceVersion,
		VerboseMode:        c.Globals.Flag.Verbose,
	})
	if err != nil {
		c.Globals.ErrLog.AddWithContext(err, map[string]interface{}{
			"Service ID":      serviceID,
			"Service Version": errors.ServiceVersion(serviceVersion),
		})
		return err
	}

	c.Input.ServiceID = serviceID
	c.Input.ServiceVersion = serviceVersion.Number

	googlepubsubs, err := c.Globals.Client.ListPubsubs(&c.Input)
	if err != nil {
		c.Globals.ErrLog.Add(err)
		return err
	}

	if !c.Globals.Verbose() {
		tw := text.NewTable(out)
		tw.AddHeader("SERVICE", "VERSION", "NAME")
		for _, googlepubsub := range googlepubsubs {
			tw.AddLine(googlepubsub.ServiceID, googlepubsub.ServiceVersion, googlepubsub.Name)
		}
		tw.Print()
		return nil
	}

	fmt.Fprintf(out, "Service ID: %s\n", c.Input.ServiceID)
	fmt.Fprintf(out, "Version: %d\n", c.Input.ServiceVersion)
	for i, googlepubsub := range googlepubsubs {
		fmt.Fprintf(out, "\tGoogle Cloud Pub/Sub %d/%d\n", i+1, len(googlepubsubs))
		fmt.Fprintf(out, "\t\tService ID: %s\n", googlepubsub.ServiceID)
		fmt.Fprintf(out, "\t\tVersion: %d\n", googlepubsub.ServiceVersion)
		fmt.Fprintf(out, "\t\tName: %s\n", googlepubsub.Name)
		fmt.Fprintf(out, "\t\tUser: %s\n", googlepubsub.User)
		fmt.Fprintf(out, "\t\tSecret key: %s\n", googlepubsub.SecretKey)
		fmt.Fprintf(out, "\t\tProject ID: %s\n", googlepubsub.ProjectID)
		fmt.Fprintf(out, "\t\tTopic: %s\n", googlepubsub.Topic)
		fmt.Fprintf(out, "\t\tFormat: %s\n", googlepubsub.Format)
		fmt.Fprintf(out, "\t\tFormat version: %d\n", googlepubsub.FormatVersion)
		fmt.Fprintf(out, "\t\tResponse condition: %s\n", googlepubsub.ResponseCondition)
		fmt.Fprintf(out, "\t\tPlacement: %s\n", googlepubsub.Placement)

	}
	fmt.Fprintln(out)

	return nil
}