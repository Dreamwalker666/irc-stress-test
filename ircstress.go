// Copyright (c) 2016 Daniel Oaks <daniel@danieloaks.net>
// released under the ISC license

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/DanielOaks/irc-stress-test/stress"
	"github.com/docopt/docopt-go"
)

func main() {
	usage := `ircstress.
Usage:
	ircstress run [--clients=<num>] [--chan-ratio=<ratio>] [--chan-join-percent=<ratio>] [--queues=<num>] [--wait] <server-details>...
	ircstress -h | --help
	ircstress --version
Options:
	--clients=<num>              The number of clients that should connect [default: 10000].
	--chan-ratio=<ratio>         How many channels there should be compared to number of clients, default 30% [default: 0.3].
	--chan-join-percent=<ratio>  How likely each client is to join one or more channels [default: 0.9].

	--queues=<num>     How many queues to run events on, limited to number of clients that exist [default: 3].
	--wait             After each action, waits for server response before continuing.
	<server-details>   Set of server details, of the format: "Name,Addr,TLS", where Addr is like "localhost:6667" and TLS is either "yes" or "no".

	-h --help          Show this screen.
	--version          Show version.`

	arguments, _ := docopt.Parse(usage, nil, true, stress.SemVer, false)

	if arguments["run"].(bool) {
		// run string
		var optionString string
		if !arguments["--wait"].(bool) {
			optionString += "not "
		}
		optionString += "waiting"

		fmt.Println(fmt.Sprintf("Running tests (%s)", optionString))

		// assemble each server's details
		servers := make(map[string]*stress.Server)
		for _, serverString := range arguments["<server-details>"].([]string) {
			serverList := strings.Split(serverString, ",")
			if len(serverList) != 3 {
				log.Fatal("Could not parse server details string:", serverString)
			}

			var isTLS bool
			if strings.ToLower(serverList[2]) == "yes" {
				isTLS = true
			} else if strings.ToLower(serverList[2]) == "no" {
				isTLS = false
			} else {
				log.Fatal("TLS must be either 'yes' or 'no', could not parse whether to enable TLS from server details:", serverString)
			}

			newServer := stress.Server{
				Name:  serverList[0],
				Addr:  serverList[1],
				IsTLS: isTLS,
			}

			fmt.Println("Running server", newServer.Name, ":", newServer.Addr)

			servers[newServer.Name] = &newServer
		}
	}
}
