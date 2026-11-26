package main

import (
	"io"

	"mc/anchor"
	"mc/substrate"
	"mc/verbs"
)

// commandAnchor is the anchoring CLI surface (D07): `anchor publish` walks
// the spine (withSpine) and `anchor verify <dir>` verifies a copied anchors
// directory fully offline — no spine, no MC_HOME (D22 requirement four).
func commandAnchor(args []string, stdin io.Reader) (any, error) {
	if len(args) == 2 && args[0] == "verify" {
		return anchorVerifyLocal(args[1])
	}
	if len(args) == 1 && args[0] == "publish" {
		var a verbs.AnchorPublishRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("anchor-publish", a)
	}
	return nil, verbs.Usagef("anchor: mc anchor <publish|verify <anchors-dir>>")
}

// anchorVerifyLocal verifies one anchors directory offline: the recomputed
// root uses the same B07 root algorithm the publisher used, so a published
// root and its head set can never disagree on the algorithm.
func anchorVerifyLocal(dir string) (any, error) {
	report, err := anchor.VerifyDir(dir, func(heads []anchor.Head) (string, int, error) {
		chains := make([]substrate.ChainHead, len(heads))
		for i, h := range heads {
			chains[i] = substrate.ChainHead{AggregateID: h.AggregateID, SeqMax: h.SeqMax, ChainDigest: h.ChainDigest}
		}
		algorithmID, leafCount, rootHex, err := substrate.ComputeRoot(chains)
		if err != nil {
			return "", 0, err
		}
		_ = algorithmID
		_ = leafCount
		return rootHex, leafCount, nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"verified": true, "report": report}, nil
}
