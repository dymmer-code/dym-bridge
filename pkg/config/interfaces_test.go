package config

import "testing"

func TestInterfaceValidation(t *testing.T) {
	valid := Tunnel{Host: "127.0.0.1", Port: 8080, ID: "wordpress", Name: "WordPress", Scheme: "http", Path: "/wp-admin/", HostHeader: "site.example.com"}
	if err := ValidateTunnels([]Tunnel{valid, {Host: "::1", Port: 3306}}); err != nil {
		t.Fatal(err)
	}
	cases := []func(*Tunnel){
		func(v *Tunnel) { v.Host = "external.example" }, func(v *Tunnel) { v.Port = 0 },
		func(v *Tunnel) { v.Scheme = "file" }, func(v *Tunnel) { v.ID = "" },
		func(v *Tunnel) { v.Name = "\n" }, func(v *Tunnel) { v.Path = "//external/" },
		func(v *Tunnel) { v.Path = "/bad\\path" }, func(v *Tunnel) { v.Path = "/%zz" },
		func(v *Tunnel) { v.HostHeader = "a\r\nb" }, func(v *Tunnel) { v.HostHeader = "host:65536" },
	}
	for i, change := range cases {
		v := valid
		change(&v)
		if err := ValidateTunnels([]Tunnel{v}); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if err := ValidateTunnels([]Tunnel{valid, valid}); err == nil {
		t.Fatal("duplicate ID accepted")
	}
	if err := ValidateTunnels(make([]Tunnel, 21)); err == nil {
		t.Fatal("too many tunnels")
	}
}
