package jmsms

// The guard against a contract that nobody keeps.
//
// A type in the IDL that no tool produces is a comment, not a contract: it
// promises a shape that nothing answers with, and the next person to read
// the IDL believes the promise. This repository had exactly that — a whole
// vocabulary of summaries that the tools never emitted, while the tools went
// on hand-assembling maps.
//
// The check is mechanical on purpose, and its BODY is shared (the
// mechanics and the failure texts of all three guards live in
// pkg/contractscan — copies drift). What THIS guard decides — its world,
// what is not a producer, and whether an enum is a declared type — is the
// Scope below, and the reasons are on the fields.

import (
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/contractscan"
)

func TestEveryContractTypeIsProduced(t *testing.T) {
	contractscan.CheckProduced(t, contractscan.Scope{
		// The scan covers THIS PLANE only. One level too high and it
		// would see the execution plane as well, and a contract that
		// only the execution plane produces would look produced here
		// too — the guard would pass for the wrong reason.
		World:    "the management plane",
		RootBase: "biz",
		// (A thrift enum is not a declared type here: "nobody produces
		// the enum" is not the failure this guard is about. The shared
		// vocabulary's guard says the opposite, and has a story for it
		// — see Scope.EnumsDeclared.)
		EnumsDeclared: false,
	})
}
