package main

import "fmt"

func errRequiredFlag(name string) error {
	return fmt.Errorf("missing required flag %s", name)
}
