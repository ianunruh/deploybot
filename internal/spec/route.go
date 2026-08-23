package spec

type Route struct {
	Port             int    `yaml:"port,omitempty"`
	Timeout          string `yaml:"timeout,omitempty"`
	GatewayNamespace string `yaml:"gatewayNamespace,omitempty"`
}

// HasRoute is true when the spec describes an HTTPRoute (timeout, port,
// gateway namespace, or any stage hostname/gateway). Controllers omit this.
func (d *Deployable) HasRoute() bool {
	r := d.Spec.Route
	if r.Port != 0 || r.Timeout != "" || r.GatewayNamespace != "" {
		return true
	}
	for _, st := range d.Spec.Stages {
		if st.Hostname != "" || st.Gateway.Name != "" || st.Gateway.SectionName != "" {
			return true
		}
	}
	return false
}
