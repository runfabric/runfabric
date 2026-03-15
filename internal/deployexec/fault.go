package deployexec

import "fmt"

type FaultConfig struct {
	FailBeforePhase string
	FailAfterPhase  string
	FailOnResource  string
	Enabled         bool
}

func (f FaultConfig) CheckBefore(phase string) error {
	if f.Enabled && f.FailBeforePhase == phase {
		return fmt.Errorf("injected failure before phase %s", phase)
	}
	return nil
}

func (f FaultConfig) CheckAfter(phase string) error {
	if f.Enabled && f.FailAfterPhase == phase {
		return fmt.Errorf("injected failure after phase %s", phase)
	}
	return nil
}

func (f FaultConfig) CheckResource(resource string) error {
	if f.Enabled && f.FailOnResource == resource {
		return fmt.Errorf("injected failure on resource %s", resource)
	}
	return nil
}
