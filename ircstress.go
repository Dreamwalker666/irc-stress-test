// Copyright (c) 2016 Daniel Oaks <daniel@danieloaks.net>
// released under the ISC license

package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"

	"io/ioutil"

	"github.com/DanielOaks/irc-stress-test/stress"
	"github.com/docopt/docopt-go"
	"github.com/olekukonko/tablewriter"
)

func startPprof(port string) {
	ps := http.Server{
		Addr: fmt.Sprintf("localhost:%s", port),
	}
	go func() {
		if err := ps.ListenAndServe(); err != nil {
			log.Fatal("couldn't start pprof", err)
		}
	}()
}

func main() {
	usage := `ircstress.
ircstress is intended to stress an IRC server through connect flooding, channel message flooding,
client message flooding and 'regular' IRC client operations. It is primarily intended to be used
during the development of IRC servers and to compare how well servers perform under load.

Usage:
	ircstress connectflood [--nicks=<file>] [--random-nicks] [--clients=<num>] [--queues=<num>] [--wait] [--pprof-port=<num>] <server-details>...
	ircstress chanflood [--nicks=<file>] [--random-nicks] [--clients=<num>] [--queues=<num>] [--wait] [--chan=<name>] [--floodsize=<num>] [--pprof-port=<num>] <server-details>...
	ircstress -h | --help
	ircstress --version

Options:
	--nicks=<file>     List to grab nicks from, separated by newlines [default: use counter].
	--random-nicks     If nicklist is given, randomise order of used nicks.
	--clients=<num>    The number of clients that should connect [default: 10000].
	--chan=<name>      Channel name to join [default: #test].
	--floodsize=<num>  Number of messages to flood with during chanflood [default: 1]

	--queues=<num>     How many queues to run events on, limited to number of clients [default: 3].
	--wait             After each action, waits for server response before continuing.
	--pprof-port=<num>     Start a pprof http endpoint for ircstress on this port
	<server-details>   Set of server details, of the format: "Name,Addr,TLS", where Addr is like "localhost:6667" and TLS is either "yes" or "no".

	-h --help          Show this screen.
	--version          Show version.

Examples:
	go run ircstress.go chanflood --clients=2000 --wait local,localhost:6667,no
		Tests a local server with 2000 clients, connecting to channel #test.`

	arguments, _ := docopt.Parse(usage, nil, true, stress.SemVer, false)

	if arguments["connectflood"].(bool) || arguments["chanflood"].(bool) {
		// get nicks
		var ns *stress.NickSelector
		if arguments["--nicks"].(string) == "use counter" {
			// do nothing
		} else {
			// load given nick list
			listBytes, err := ioutil.ReadFile(arguments["--nicks"].(string))
			if err != nil {
				log.Fatal("Could not load nickList:", err.Error())
			}
			ns = stress.NickSelectorFromList(string(listBytes))
			if arguments["--random-nicks"].(bool) {
				ns.RandomNickOrder = true
			}
		}

		port := arguments["--pprof-port"]
		if port != nil {
			startPprof(port.(string))
		}

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
				Name: serverList[0],
				Conn: stress.ServerConnectionDetails{
					Address: serverList[1],
					IsTLS:   isTLS,
				},
			}

			fmt.Println("Testing server", newServer.Name, "at", newServer.Conn.Address)

			servers[newServer.Name] = &newServer
		}

		clientCount, err := strconv.Atoi(arguments["--clients"].(string))
		if err != nil || clientCount < 1 {
			log.Fatal("Invalid number of clients:", arguments["--clients"].(string))
		}

		// create event queues
		eventQueues := make([]stress.EventQueue, clientCount)
		var deliberateDisconnects int

		var floodLines []string
		if arguments["chanflood"].(bool) {
			floodCount, err := strconv.Atoi(arguments["--floodsize"].(string))
			if err != nil {
				floodCount = 1
			}
			floodLines = make([]string, floodCount)
			channelName :=arguments["--chan"].(string)
			for i := 0; i < len(floodLines); i++ {
				floodLines[i] = fmt.Sprintf("PRIVMSG %s :Test string %d to flood with here\r\n", channelName, i)
			}
		}

		for i := 0; i < clientCount; i++ {
			var newClient *stress.Client
			if ns == nil {
				newClient = &stress.Client{
					Nick: fmt.Sprintf("cli%d", i),
				}
			} else {
				newClient = &stress.Client{
					Nick: ns.GetNick(),
				}
			}

			// for now we'll just have one event list per client for simplicity
			events := stress.NewEventQueue(i)
			events.Events = append(events.Events, stress.Event{
				Type: stress.ETConnect,
			})

			// send NICK+USER
			// events.Events = append(events.Events, stress.Event{
			// 	Type:   stress.ETLine,
			// 	Line:   fmt.Sprintf("CAP END\r\n", newClient.Nick),
			// })
			events.Events = append(events.Events, stress.Event{
				Type: stress.ETLine,
				Line: fmt.Sprintf("NICK %s\r\n", newClient.Nick),
			})
			events.Events = append(events.Events, stress.Event{
				Type: stress.ETLine,
				Line: "USER test 0 * :I am a cool person!\r\n",
			})

			if arguments["chanflood"].(bool) {
				events.Events = append(events.Events, stress.Event{
					Type: stress.ETLine,
					Line: fmt.Sprintf("JOIN %s\r\n", arguments["--chan"].(string)),
				})
				for _, line := range floodLines {
					events.Events = append(events.Events, stress.Event{
						Type: stress.ETLine,
						Line: line,
					})
				}
				events.Events = append(events.Events, stress.Event{
					Type: stress.ETPing,
				})
			}

			//TODO(dan): send NICK/USER
			events.Events = append(events.Events, stress.Event{
				Type: stress.ETDisconnect,
			})
			deliberateDisconnects++

			eventQueues[i] = events
		}

		// run for each server
		for name, server := range servers {
			fmt.Println("Testing", name)
			server.ClientsReadyToDisconnect.Add(deliberateDisconnects)
			server.ClientsFinished.Add(clientCount)

			// start each event queue
			for _, events := range eventQueues {
				time.Sleep(time.Millisecond * 3)
				go events.Run(server)
			}

			// wait for each of them to be finished
			server.ClientsFinished.Wait()

			data := [][]string{
				[]string{"Total Clients", strconv.Itoa(clientCount)},
				[]string{"Successful Clients", strconv.Itoa(int(server.Succeeded()))},
			}

			table := tablewriter.NewWriter(os.Stdout)
			for _, v := range data {
				table.Append(v)
			}
			table.Render() // Send output
		}
	}
}
