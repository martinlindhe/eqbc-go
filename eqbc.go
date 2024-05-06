package eqbc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type EQBC struct {
	sync.RWMutex
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

// register clientID == name mapping.
func (eqbc *EQBC) registerClient(con net.Conn, clientID uint64, clientName string) {
	eqbc.Lock()
	defer eqbc.Unlock()
	eqbc.clients[clientID] = eqbcClient{con, clientName}
}

func (eqbc *EQBC) destroyClient(clientName string, clientID uint64) {
	eqbc.leaveAllChannels(clientName)
	eqbc.Lock()
	defer eqbc.Unlock()
	delete(eqbc.clients, clientID)
}

// Remove client from all their channels.
func (eqbc *EQBC) leaveAllChannels(username string) {
	eqbc.Lock()
	defer eqbc.Unlock()
	for channel, members := range eqbc.channelMembers {
		list := []string{}
		for _, n := range members {
			if n != username {
				list = append(list, n)
			}
		}
		eqbc.channelMembers[channel] = list
	}
}

// add user to the listed channels.
func (eqbc *EQBC) joinChannels(username string, channels []string) {
	eqbc.Lock()
	defer eqbc.Unlock()
	for _, c := range channels {
		eqbc.channelMembers[c] = append(eqbc.channelMembers[c], username)
	}
}

func (eqbc *EQBC) getClientNames() (res []string) {
	eqbc.RLock()
	defer eqbc.RUnlock()
	for _, c := range eqbc.clients {
		res = append(res, c.name)
	}
	return
}

func (eqbc *EQBC) getClientName(clientID uint64) string {
	eqbc.RLock()
	defer eqbc.RUnlock()
	for id, c := range eqbc.clients {
		if id == clientID {
			return c.name
		}
	}
	return ""
}

func (eqbc *EQBC) getClientByName(name string) *eqbcClient {
	eqbc.RLock()
	defer eqbc.RUnlock()
	for _, c := range eqbc.clients {
		if strings.EqualFold(name, c.name) {
			return &c
		}
	}
	return nil
}

func (eqbc *EQBC) broadcast(pkt string) {
	eqbc.RLock()
	defer eqbc.RUnlock()
	for clientID, c := range eqbc.clients {
		if _, err := c.con.Write([]byte(pkt)); err != nil {
			eqbc.Log(fmt.Sprintf("[% 4d] failed to respond to client: %v", clientID, err))
		}
	}
}

// Send payload to all other clients, excluding `senderName`.
func (eqbc *EQBC) broadcastOthers(clientID uint64, senderName, payload string) {
	eqbc.RLock()
	defer eqbc.RUnlock()
	for _, c := range eqbc.clients {
		if c.name == senderName {
			continue
		}
		pkt := "<" + senderName + "> " + c.name + " " + payload + "\n"
		if eqbc.verbose {
			eqbc.Log(fmt.Sprintf("[% 4d] TO %s: %s", clientID, c.name, strings.TrimSpace(pkt)))
		}
		_ = eqbc.sendTo(c.name, pkt)
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
	eqbc.RLock()
	defer eqbc.RUnlock()
	for channel, members := range eqbc.channelMembers {
		if strings.EqualFold(channel, channelName) {
			return members, nil
		}
	}
	return nil, fmt.Errorf("%s: No such channel", channelName)
}

func (eqbc *EQBC) pingClient(con io.Writer, clientID uint64) {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		_, err := con.Write([]byte("\tPING\n"))
		if err != nil {
			eqbc.Log(fmt.Sprintf("[% 4d] PING failed: %s, removing client", clientID, err.Error()))
			clientName := eqbc.getClientName(clientID)
			eqbc.destroyClient(clientName, clientID)
			return
		}
	}
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

	clientName, password, err := parseLoginPacket(login)
	if err != nil {
		eqbc.Log("LOGIN error: " + err.Error())
		return
	}
	if password != eqbc.password {
		eqbc.Log(fmt.Sprintf("[% 4d] error: invalid password", clientID))
		_, _ = con.Write([]byte("-- Invalid server password.\n"))
		return
	}

	eqbc.registerClient(con, clientID, clientName)
	eqbc.nbJoin(clientName)
	eqbc.Log(fmt.Sprintf("[% 4d] [+g+]%s joined", clientID, clientName))

	go eqbc.pingClient(con, clientID)

	for {
		msgType, err := readBytes(clientReader)
		if err != nil {
			// in case of network error, broadcast disconnect to all
			eqbc.Log(fmt.Sprintf("[% 4d] [+r+]%s disconnected (error: %v)", clientID, clientName, err))
			eqbc.destroyClient(clientName, clientID)
			eqbc.nbQuit(clientName)
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
			payload = strings.ReplaceAll(payload, "\\\\", "") // Suppress the backslash \\
			eqbc.Log(fmt.Sprintf("[% 4d] TELL: %s", clientID, payload))

			pos := strings.Index(payload, " ")
			if pos == -1 {
				// invalid syntax
				eqbc.Log(fmt.Sprintf("[% 4d] TELL invalid syntax: %s", clientID, payload))
				continue
			}

			receiver := payload[0:pos]
			content := payload[pos+1:]
			pkt := "[" + clientName + "] " + content + "\n"
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
			payload = strings.ReplaceAll(payload, "\\\\", "") // Suppress the backslash \\
			eqbc.Log(fmt.Sprintf("[% 4d] BCA %s", clientID, payload))

			eqbc.broadcastOthers(clientID, clientName, payload)

		case "\tNBMSG\n":
			// netbots message
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1d): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))
			payload = strings.ReplaceAll(payload, "\\\\", "") // Suppress the backslash \\

			// broadcast NBPKT
			pkt := "\tNBPKT:" + clientName + ":" + payload + "\n"
			if eqbc.verbose {
				// eqbc.Log(fmt.Sprintf("[% 4d] NBMSG: %s", clientID, payload))
				eqbc.Log(fmt.Sprintf("[% 4d] -> %s", clientID, strings.TrimSpace(pkt)))
			}
			eqbc.broadcast(pkt)
			continue

		case "\tNAMES\n":
			// respond with all connected client names
			names := strings.Join(eqbc.getClientNames(), ", ")
			eqbc.Log(fmt.Sprintf("[% 4d] requested names: %s", clientID, names))
			_, _ = con.Write([]byte("-- Names: " + names + ".\n"))

		case "\tCHANNELS\n":
			// set the list of channels to receive tells from
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1e): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))
			payload = strings.ReplaceAll(payload, "\\\\", "") // Suppress the backslash \\
			channels := strings.Split(payload, " ")

			eqbc.leaveAllChannels(clientName)
			eqbc.joinChannels(clientName, channels)

			_, _ = con.Write([]byte(clientName + " joined channels " + payload + ".\n"))

		case "\tDISCONNECT\n":
			eqbc.Log(fmt.Sprintf("[% 4d] [+r+]%s disconnected", clientID, clientName))
			eqbc.destroyClient(clientName, clientID)
			eqbc.nbQuit(clientName)
			return

		case "\tPONG\n":
			// we initiate pings so just swallow these
			data, err := readBytes(clientReader)
			if err != nil {
				eqbc.Log(fmt.Sprintf("[% 4d] error(1b): %v", clientID, err))
				return
			}
			payload := strings.TrimSpace(string(data))
			if payload != "" {
				eqbc.Log(fmt.Sprintf("[% 4d] Unexpected PONG data from client: %s", clientID, payload))
			}
			continue

		default:
			// /bc commands: broadcast to all clients
			pkt := "<" + clientName + "> " + string(msgType)
			eqbc.Log(fmt.Sprintf("[% 4d] %s", clientID, strings.TrimSpace(pkt)))
			eqbc.broadcast(pkt)
		}
	}
}

// broadcast NBJOIN.
func (eqbc *EQBC) nbJoin(username string) {
	pkt := "\tNBJOIN=" + username + "\n" +
		"\tNBCLIENTLIST=" + strings.Join(eqbc.getClientNames(), " ") + "\n"
	eqbc.broadcast(pkt)
}

// broadcast NBQUIT.
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
