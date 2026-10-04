package main

import "testing"

func TestPolish(t *testing.T) {
	in := "## mate x\n" +
		"\n" +
		"### Synopsis\n" +
		"\n" +
		"Run on <path>.\n" +
		"Examples:\n" +
		"  mate x a      # one\n" +
		"\n" +
		"  mate x b\n" +
		"Sources:\n" +
		"  - <source>/.mate.yaml\n" +
		"    continued\n" +
		"\n" +
		"```\n" +
		"mate x <path> [flags]\n" +
		"```\n" +
		"\n" +
		"### Options\n" +
		"\n" +
		"```\n" +
		"  -h, --help   help for x\n" +
		"```\n"

	want := "# mate x\n" +
		"\n" +
		"## Synopsis\n" +
		"\n" +
		"Run on &lt;path>.\n" +
		"Examples:\n" +
		"\n" +
		"```\n" +
		"mate x a      # one\n" +
		"\n" +
		"mate x b\n" +
		"```\n" +
		"\n" +
		"Sources:\n" +
		"\n" +
		"- &lt;source>/.mate.yaml\n" +
		"  continued\n" +
		"\n" +
		"```\n" +
		"mate x <path> [flags]\n" +
		"```\n" +
		"\n" +
		"## Options\n" +
		"\n" +
		"```\n" +
		"  -h, --help   help for x\n" +
		"```\n"

	if got := polish(in); got != want {
		t.Errorf("polish mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
