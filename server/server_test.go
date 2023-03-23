package server

import (
	"testing"

	"github.com/miekg/dns"
)

func Test_server_ServeDNS(t *testing.T) {
	type fields struct {
		listen string
	}
	type args struct {
		w dns.ResponseWriter
		r *dns.Msg
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		//Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{
				listen: tt.fields.listen,
			}
			s.ServeDNS(tt.args.w, tt.args.r)
		})
	}
}
