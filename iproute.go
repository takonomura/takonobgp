package main

import (
	"log"
	"os/exec"
	"strings"
)

func ipRoute(args ...string) error {
	log.Printf("execute: ip route %s", strings.Join(args, " "))
	cmd := exec.Command("ip", append([]string{"route"}, args...)...)
	result, err := cmd.CombinedOutput()
	if len(result) > 0 {
		log.Printf("ip route: %s", string(result))
	}
	if err != nil {
		return err
	}
	return nil
}
