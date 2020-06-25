package pod

import (
	"fmt"
)

func pkgerror(err error) error {
	if err != nil {
		err = fmt.Errorf("pod: %v", err)
	}
	return err
}

// Name gets the name of the pod associated with a hash
func Name(hash string) string {
	return "kdo-" + hash
}
