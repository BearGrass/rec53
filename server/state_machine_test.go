package server

import (
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestChange(t *testing.T) {
	type args struct {
		stm stateMachine
	}
	tests := []struct {
		name    string
		args    args
		want    *dns.Msg
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Change(tt.args.stm)
			if (err != nil) != tt.wantErr {
				t.Errorf("Change() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Change() = %v, want %v", got, tt.want)
			}
		})
	}
}
