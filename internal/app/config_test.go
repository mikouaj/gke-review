package app

import (
	"context"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	c := Config{}
	err := c.Load(context.Background())
	if err != nil {
		t.Errorf("err is not nil; want nil")
	}
	if !reflect.DeepEqual(c.out.w, os.Stdout) {
		t.Errorf("c.out.w is not os.stdout")
	}
	if c.gke == nil {
		t.Errorf("c.gke is nil; want gke.GKEClient")
	}
	err = c.Close()
	if err != nil {
		t.Errorf("err is not nil; want nil")
	}
}

func TestConfigLoad_silent(t *testing.T) {
	c := Config{SilentMode: true}
	err := c.Load(context.Background())
	if err != nil {
		t.Errorf("err is not nil; want nil")
	}
	if !reflect.DeepEqual(c.out.w, io.Discard) {
		t.Errorf("c.out.w is not io.Discard")
	}
	err = c.Close()
	if err != nil {
		t.Errorf("err is not nil; want nil")
	}
}
