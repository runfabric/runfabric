package cli

import "fmt"

// stubMsg prints a consistent "implementation pending" message for stub commands.
func stubMsg(cmd string, keyValues ...string) {
	if len(keyValues) == 0 {
		fmt.Printf("%s: implementation pending\n", cmd)
		return
	}
	args := ""
	for i := 0; i < len(keyValues); i += 2 {
		if i > 0 {
			args += " "
		}
		k := keyValues[i]
		v := ""
		if i+1 < len(keyValues) {
			v = keyValues[i+1]
		}
		args += fmt.Sprintf("%s=%s", k, v)
	}
	fmt.Printf("%s: implementation pending (%s)\n", cmd, args)
}
