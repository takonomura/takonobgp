package main

import (
	"log"
	"os/exec"
)

func ipRoute(args ...string) error {
	cmd := exec.Command("ip", append([]string{"route"}, args...)...)
	result, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	if len(result) > 0 {
		log.Printf("ip route: %s", string(result))
	}
	return nil
}
