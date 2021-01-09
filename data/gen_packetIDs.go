// gen_packetIDs.go generates the enumeration of packet IDs used on the wire.

//+build ignore

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/iancoleman/strcase"
)

const (
	protocolURL = "https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/1.16.2/protocol.json"
)

// unnest is a utility function to unpack a value from a nested map, given
// an arbitrary set of keys to reach through.
func unnest(input map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	for _, k := range keys {
		sub, ok := input[k]
		if !ok {
			return nil, fmt.Errorf("key %q not found", k)
		}
		next, ok := sub.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("key %q was %T, expected a string map", k, sub)
		}
		input = next
	}
	return input, nil
}

type duplexMappings struct {
	Clientbound map[string]string
	Serverbound map[string]string
}

func (m *duplexMappings) EnsureUniqueNames() {
	// Assemble a slice of keys to check across both maps, because we cannot
	// mutate a map while iterating it.
	clientKeys := make([]string, 0, len(m.Clientbound))
	for k, _ := range m.Clientbound {
		clientKeys = append(clientKeys, k)
	}

	for _, k := range clientKeys {
		if _, alsoServerKey := m.Serverbound[k]; alsoServerKey {
			cVal, sVal := m.Clientbound[k], m.Serverbound[k]
			delete(m.Clientbound, k)
			delete(m.Serverbound, k)
			m.Clientbound[k+"Clientbound"] = cVal
			m.Serverbound[k+"Serverbound"] = sVal
		}
	}
}

// unpackMapping returns the set of packet IDs and their names for a given
// game state.
func unpackMapping(data map[string]interface{}, gameState string) (duplexMappings, error) {
	out := duplexMappings{
		Clientbound: make(map[string]string),
		Serverbound: make(map[string]string),
	}

	info, err := unnest(data, gameState, "toClient", "types")
	if err != nil {
		return duplexMappings{}, err
	}
	pType := info["packet"].([]interface{})[1].([]interface{})[0].(map[string]interface{})["type"]
	mappings := pType.([]interface{})[1].(map[string]interface{})["mappings"].(map[string]interface{})
	for k, v := range mappings {
		out.Clientbound[strcase.ToCamel(v.(string))] = k
	}
	info, err = unnest(data, gameState, "toServer", "types")
	if err != nil {
		return duplexMappings{}, err
	}
	pType = info["packet"].([]interface{})[1].([]interface{})[0].(map[string]interface{})["type"]
	mappings = pType.([]interface{})[1].(map[string]interface{})["mappings"].(map[string]interface{})
	for k, v := range mappings {
		out.Serverbound[strcase.ToCamel(v.(string))] = k
	}

	return out, nil
}

type protocolIDs struct {
	Login  duplexMappings
	Play   duplexMappings
	Status duplexMappings
	// Handshake state has no packets
}

func (p protocolIDs) MaxLen() int {
	var max int
	for _, m := range []duplexMappings{p.Login, p.Play, p.Status} {
		for k, _ := range m.Clientbound {
			if len(k) > max {
				max = len(k)
			}
		}
		for k, _ := range m.Serverbound {
			if len(k) > max {
				max = len(k)
			}
		}
	}
	return max
}

func downloadInfo() (*protocolIDs, error) {
	resp, err := http.Get(protocolURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var out protocolIDs
	if out.Login, err = unpackMapping(data, "login"); err != nil {
		return nil, fmt.Errorf("login: %v", err)
	}
	out.Login.EnsureUniqueNames()
	if out.Play, err = unpackMapping(data, "play"); err != nil {
		return nil, fmt.Errorf("play: %v", err)
	}
	out.Play.EnsureUniqueNames()
	if out.Status, err = unpackMapping(data, "status"); err != nil {
		return nil, fmt.Errorf("play: %v", err)
	}
	out.Status.EnsureUniqueNames()

	return &out, nil
}

func main() {
	pIDs, err := downloadInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	maxLen := pIDs.MaxLen()

	f, err := os.Create("packetIDs.go")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Fprintln(f, "// This file is automatically generated by gen_packetIDs.go. DO NOT EDIT.")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "package data")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "//go:generate go run gen_packetIDs.go")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "// PktID represents a packet ID used in the minecraft protocol.")
	fmt.Fprintln(f, "type PktID int32")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "// Valid PktID values.")
	fmt.Fprintln(f, "const (")

	fmt.Fprintln(f, "  // Clientbound packets for connections in the login state.")
	for k, v := range pIDs.Login.Clientbound {
		fmt.Fprintf(f, "  %s%s PktID = %s\n", k, strings.Repeat(" ", maxLen-len(k)), v)
	}
	fmt.Fprintln(f, "  // Serverbound packets for connections in the login state.")
	for k, v := range pIDs.Login.Serverbound {
		fmt.Fprintf(f, "  %s%s PktID = %s\n", k, strings.Repeat(" ", maxLen-len(k)), v)
	}
	fmt.Fprintln(f)

	fmt.Fprintln(f, "  // Clientbound packets for connections in the play state.")
	for k, v := range pIDs.Play.Clientbound {
		fmt.Fprintf(f, "  %s%s PktID = %s\n", k, strings.Repeat(" ", maxLen-len(k)), v)
	}
	fmt.Fprintln(f, "  // Serverbound packets for connections in the play state.")
	for k, v := range pIDs.Play.Serverbound {
		fmt.Fprintf(f, "  %s%s PktID = %s\n", k, strings.Repeat(" ", maxLen-len(k)), v)
	}
	fmt.Fprintln(f)

	fmt.Fprintln(f, "  // Clientbound packets used to respond to ping/status requests.")
	for k, v := range pIDs.Status.Clientbound {
		fmt.Fprintf(f, "  %s%s PktID = %s\n", k, strings.Repeat(" ", maxLen-len(k)), v)
	}
	fmt.Fprintln(f, "  // Serverbound packets used to ping or read server status.")
	for k, v := range pIDs.Status.Serverbound {
		fmt.Fprintf(f, "  %s%s PktID = %s\n", k, strings.Repeat(" ", maxLen-len(k)), v)
	}
	fmt.Fprintln(f)

	fmt.Fprintln(f, ")")
}