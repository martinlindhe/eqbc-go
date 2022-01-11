package main

import (
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"

	"github.com/martinlindhe/eqbc-go"
)

var args struct {
	Host        string `help:"Listen to host." default:"0.0.0.0"`
	Port        int    `help:"Listen to port." default:"2112"`
	Password    string `help:"Server password." default:""`
	Verbose     bool   `short:"v" help:"Be more verbose."`
	NoTimestamp bool   `help:"Hide timestamps from log."`
	NoColor     bool   `help:"Disable color output."`
}

func main() {
	_ = kong.Parse(&args,
		kong.Name("eqbc-go"))

	if args.NoColor {
		color.NoColor = true // disables colorized output
	}

	listenAddr := fmt.Sprintf("%s:%d", args.Host, args.Port)

	server := eqbc.NewServer(eqbc.ServerConfig{
		Verbose:     args.Verbose,
		Password:    args.Password,
		NoTimestamp: args.NoTimestamp,
	})

	banner := " --==> eqbc-go LISTENING AT " + listenAddr
	if args.Password != "" {
		banner += " (password is '" + args.Password + "')"
	}
	server.Log(banner)

	err := server.Listen(listenAddr)
	if err != nil {
		fmt.Println("ERROR: ", err)
		return
	}
}
