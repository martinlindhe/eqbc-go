package eqbc

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/fatih/color"
)

type EQBC struct {
	clients        map[uint64]eqbcClient
	verbose        bool
	noTimestamp    bool
	password       string
	clientCounter  uint64
	channelMembers map[string][]string
}

type eqbcClient struct {
	con  net.Conn
	name string
}

type ServerConfig struct {
	Verbose     bool
	NoTimestamp bool
	Password    string
}

func NewServer(cfg ServerConfig) *EQBC {
	return &EQBC{
		clients:        make(map[uint64]eqbcClient),
		verbose:        cfg.Verbose,
		noTimestamp:    cfg.NoTimestamp,
		password:       cfg.Password,
		channelMembers: make(map[string][]string),
	}
}

func (eqbc *EQBC) Listen(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		con, err := l.Accept()
		if err != nil {
			eqbc.Log("accept error: " + err.Error())
			continue
		}
		go eqbc.handleConnection(con)
	}
}

func (eqbc *EQBC) Log(s string) {
	if !eqbc.noTimestamp {
		fmt.Print("[" + time.Now().Format("15:04:05") + "]")
	}
	fmt.Println(colorize(s))
}

// register clientID == name mapping
func (eqbc *EQBC) registerClient(con net.Conn, clientID uint64, name string) {
	eqbc.clients[clientID] = eqbcClient{con, name}
}

func (eqbc *EQBC) destroyClient(clientID uint64) {
	eqbc.leaveAllChannels(clientID)
	delete(eqbc.clients, clientID)
}

// remove client from all their channels
func (eqbc *EQBC) leaveAllChannels(clientID uint64) {
	name := eqbc.getClientName(clientID)
	for channel, members := range eqbc.channelMembers {
		list := []string{}
		for _, n := range members {
			if n != name {
				list = append(list, n)
			}
		}
		eqbc.Log(fmt.Sprintf("[% 4d] destroyClient CHANNEL %s ADJUSTED FROM %s TO %s", clientID, channel, strings.Join(members, " "), strings.Join(list, " ")))
		eqbc.channelMembers[channel] = list
	}
}

func (eqbc *EQBC) getClientNames() (res []string) {
	for _, c := range eqbc.clients {
		res = append(res, c.name)
	}
	return
}

func (eqbc *EQBC) getClientName(clientID uint64) string {
	for id, c := range eqbc.clients {
		if id == clientID {
			return c.name
		}
	}
	return ""
}

func (eqbc *EQBC) getClientByName(name string) *eqbcClient {
	for _, c := range eqbc.clients {
		if strings.EqualFold(name, c.name) {
			return &c
		}
	}
	return nil
}

func (eqbc *EQBC) broadcast(pkt string) {
	for clientID, c := range eqbc.clients {
		if _, err := c.con.Write([]byte(pkt)); err != nil {
			eqbc.Log(fmt.Sprintf("[% 4d] failed to respond to client: %v", clientID, err))
		}
	}
}

func (eqbc *EQBC) sendTo(receiver, pkt string) error {
	// send to a single client
	c := eqbc.getClientByName(receiver)
	if c != nil {
		if _, err := c.con.Write([]byte(pkt)); err != nil {
			return err
		}
		return nil
	}

	// send to all members of a channel
	members, err := eqbc.getChannelMembers(receiver)
	if err != nil {
		return fmt.Errorf("%s: No such name", receiver)
	}

	for _, member := range members {
		c := eqbc.getClientByName(member)
		if c == nil {
			eqbc.Log("failed to find client: " + member)
			continue
		}
		if _, err := c.con.Write([]byte(pkt)); err != nil {
			eqbc.Log("failed to send to client: " + err.Error())
			continue
		}
	}
	return nil
}

func (eqbc *EQBC) getChannelMembers(channelName string) ([]string, error) {
	for channel, members := range eqbc.channelMembers {
		if strings.EqualFold(channel, channelName) {
			return members, nil
		}
	}
	return nil, fmt.Errorf("%s: No such channel", channelName)
}

func (eqbc *EQBC) handleConnection(con net.Conn) {

	defer con.Close()

	eqbc.clientCounter++
	clientID := eqbc.clientCounter

	eqbc.Log(fmt.Sprintf("[% 4d] [+y+]Client %s connected", clientID, con.RemoteAddr().String()))

	clientReader := bufio.NewReader(con)

	login, err := clientReader.ReadBytes(';')
	if err != nil {
		eqbc.Log("LOGIN packet error: " + err.Error())
		return
	}

	username, password, err := parseLoginPacket(login)
	if err != nil {
		eqbc.Log("LOGIN error: " + err.Error())
		return
	}
	if password != eqbc.password {
		eqbc.Log(fmt.Sprintf("[% 4d] error: invalid password", clientID))
		con.Write([]byte("-- Invalid server password.\n"))
		return
	}

	eqbc.registerClient(con, clientID, username)
	eqbc.nbJoin(username)
	eqbc.Log(fmt.Sprintf("[% 4d] [+g+]%s joined", clientID, username))

	for {
		msgType, err := readBytes(clientReader)
		if err != nil {
			// in case of network error, broadcast disconnect to all
			eqbc.Log(fmt.Sprintf("[% 4d] [+r+]%s disconnected (error: %v)", clientID, username, err))
			eqbc.destroyClient(clientID)
			eqbc.nbQuit(username)
			return
		}

		switch string(msgType) {
		case "\tLOCALECHO 1\n":
			// XXX not implemented
			// 1 = echo my messages back to me (default). 0 = mute

			/*
				fmt.Printf("[% 4d] LOCALECHO 1\n", clientID)

				// original implementation replies like this:
				if _, err = con.Write([]byte("-- Local Echo: ON\n")); err != nil {
					fmt.Printf("[% 4d] failed to respond to client: %v\n", clientID, err)
				}
			*/
			continue

		case "\tTELL\n":
			// /bct username command
			// /bct channel command
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1b): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))
			eqbc.Log(fmt.Sprintf("[% 4d] TELL: %s", clientID, payload))

			pos := strings.Index(payload, " ")
			if pos == -1 {
				// invalid syntax
				eqbc.Log(fmt.Sprintf("[% 4d] TELL invalid syntax: %s", clientID, payload))
				continue
			}

			receiver := payload[0:pos]
			content := payload[pos+1:]
			pkt := "[" + username + "] " + content + "\n"
			eqbc.Log(fmt.Sprintf("[% 4d] TO %s: %s", clientID, receiver, strings.TrimSpace(pkt)))
			if err := eqbc.sendTo(receiver, pkt); err != nil {
				// if receiver is not connected, return an error to client
				if _, err = con.Write([]byte("-- " + err.Error() + ".\n")); err != nil {
					eqbc.Log(fmt.Sprintf("[% 4d] failed to respond to client: %v", clientID, err))
				}
			}

		case "\tMSGALL\n":
			// /bca content
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1c): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))
			eqbc.Log(fmt.Sprintf("[% 4d] BCA %s", clientID, payload))

			// send command to all other clients
			for _, c := range eqbc.clients {
				if c.name == username {
					continue
				}
				pkt := "<" + username + "> " + c.name + " " + payload + "\n"
				if eqbc.verbose {
					eqbc.Log(fmt.Sprintf("[% 4d] TO %s: %s", clientID, c.name, strings.TrimSpace(pkt)))
				}
				eqbc.sendTo(c.name, pkt)
			}

		case "\tNBMSG\n":
			// netbots message
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1d): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))

			// broadcast NBPKT
			pkt := "\tNBPKT:" + username + ":" + payload + "\n"
			if eqbc.verbose {
				//eqbc.Log(fmt.Sprintf("[% 4d] NBMSG: %s", clientID, payload))
				eqbc.Log(fmt.Sprintf("[% 4d] -> %s", clientID, strings.TrimSpace(pkt)))
			}
			eqbc.broadcast(pkt)
			continue

		case "\tNAMES\n":
			// respond with all connected client names
			names := strings.Join(eqbc.getClientNames(), ", ")
			eqbc.Log(fmt.Sprintf("[% 4d] requested names: %s", clientID, names))
			con.Write([]byte("-- Names: " + names + ".\n"))

		case "\tCHANNELS\n":
			// set the list of channels to receive tells from
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1e): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))
			channels := strings.Split(payload, " ")

			// remove user from all channels
			eqbc.leaveAllChannels(clientID)

			// add user to the listed channels
			for _, c := range channels {
				eqbc.channelMembers[c] = append(eqbc.channelMembers[c], username)
			}

			con.Write([]byte(username + " joined channels " + payload + ".\n"))

		case "\tDISCONNECT\n":
			eqbc.Log(fmt.Sprintf("[% 4d] [+r+]%s disconnected", clientID, username))
			eqbc.destroyClient(clientID)
			eqbc.nbQuit(username)
			return

		default:
			// /bc commands: broadcast to all clients
			pkt := "<" + username + "> " + string(msgType)
			eqbc.Log(fmt.Sprintf("[% 4d] %s", clientID, strings.TrimSpace(pkt)))
			eqbc.broadcast(pkt)
		}
	}
}

func colorize(s string) string {
	for {
		pos := strings.Index(s, "[+")
		if pos == -1 {
			break
		}

		next := strings.Index(s, "+]")
		if next < pos {
			fmt.Printf("ERROR: invalid colorize string '%s'\n", s)
			return s
		}

		before := s[0:pos]
		after := s[next+2:]
		token := s[pos+2 : next]

		col := getColor(token)
		s = before + col.SprintFunc()(after)
	}

	return s
}

func getColor(token string) *color.Color {
	switch token {
	case "b", "B":
		return color.New(color.FgBlack)

	case "g":
		return color.New(color.FgHiGreen)

	case "y":
		return color.New(color.FgHiYellow)

	case "r":
		return color.New(color.FgHiRed)

	case "G": // dark green
		return color.New(color.FgGreen)

	case "Y": // dark grey
		return color.New(color.FgHiBlack)

	case "R": // dark red
		return color.New(color.FgRed)

	case "w":
		return color.New(color.FgHiWhite)

	case "W": // light grey
		return color.New(color.FgWhite)
	}
	fmt.Printf("ERROR: unhandled color code '%s'\n", token)
	return color.New(color.FgWhite)
}

// broadcast NBJOIN
func (eqbc *EQBC) nbJoin(username string) {
	pkt := "\tNBJOIN=" + username + "\n" +
		"\tNBCLIENTLIST=" + strings.Join(eqbc.getClientNames(), " ") + "\n"
	eqbc.broadcast(pkt)
}

// broadcast NBQUIT
func (eqbc *EQBC) nbQuit(username string) {
	pkt := "\tNBQUIT=" + username + "\n" +
		"\tNBCLIENTLIST=" + strings.Join(eqbc.getClientNames(), " ") + "\n"
	eqbc.broadcast(pkt)
}

func readBytes(reader *bufio.Reader) ([]byte, error) {
	msgType, err := reader.ReadBytes('\n')
	switch err.(type) {
	case *net.OpError:
		return nil, fmt.Errorf("client disconnected")
	}
	return msgType, err
}

// parses login packets in the format of "LOGIN=name;" or "LOGIN:pwd=name;"
func parseLoginPacket(b []byte) (username string, password string, err error) {
	if len(b) < len("LOGIN=n;") {
		return "", "", fmt.Errorf("input too short")
	}
	if string(b[0:5]) != "LOGIN" {
		return "", "", fmt.Errorf("invalid prefix")
	}

	end := bytes.Index(b, []byte(";"))
	if end == -1 {
		return "", "", fmt.Errorf("end delimiter missing")
	}

	mid := b[len("LOGIN"):end]

	switch mid[0] {
	case '=':
		// "LOGIN=name;" format
		return string(mid[1:]), "", nil

	case ':':
		// "LOGIN:password=name;" format
		passwordSep := bytes.Index(mid, []byte("="))
		if passwordSep == -1 {
			return "", "", fmt.Errorf("password separator missing")
		}
		pwd := mid[1:passwordSep]
		return string(mid[passwordSep+1:]), string(pwd), nil
	}

	return "", "", fmt.Errorf("invalid separator")
}
