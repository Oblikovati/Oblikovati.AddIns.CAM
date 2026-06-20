// SPDX-License-Identifier: GPL-2.0-only

// Command camrun drives the CAM engine headlessly against a recorded host: it answers the
// geometry queries with a canned part (a plate with holes) and runs the full drilling flow
// (detect → toolpath → post), writing the generated G-code to a file. It lets the pipeline
// be exercised and inspected end-to-end without a running Oblikovati host — the offline
// oracle harness, mirroring the FEMM bridge's cmd/femmfield.
//
//	go run ./cmd/camrun -out /tmp/cam-holes.nc -post grbl
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"oblikovati.org/api/wire"
	"oblikovati.org/cam/bridge"
)

// plateHost is a HostCaller that answers the engine's read queries with a fixed part — a
// 100×60×10 mm plate (cm in the database) carrying three vertical holes — and absorbs the
// graphics/status writes. For milling ops it answers a mid-height section with the plate's
// rectangular outline. It stands in for a live host so the pipeline runs offline.
type plateHost struct{}

func (plateHost) Call(method string, _ []byte) ([]byte, error) {
	switch method {
	case wire.MethodModelReferenceKeys:
		return json.Marshal(wire.ReferenceKeysResult{Bodies: []wire.BodyTopology{{Faces: []wire.TopologyRef{
			{Key: "f0", Kind: "plane", Point: []float64{5, 3, 1}},
			{Key: "h1", Kind: "cylinder", Point: []float64{2, 2, 0.5}}, // cm
			{Key: "h2", Kind: "cylinder", Point: []float64{8, 2, 0.5}},
			{Key: "h3", Kind: "cylinder", Point: []float64{5, 4, 0.5}},
		}}}})
	case wire.MethodBodyRangeBox:
		return json.Marshal(wire.BodyRangeBoxResult{Min: []float64{0, 0, 0}, Max: []float64{10, 6, 1}}) // cm
	case wire.MethodBrepSectionWithPlane:
		// The plate's rectangular outline at mid-height (cm, closed loop).
		return json.Marshal(wire.BrepWiresResult{Wires: []wire.WirePolyline{{
			Points: []float64{0, 0, 0.5, 10, 0, 0.5, 10, 6, 0.5, 0, 6, 0.5, 0, 0, 0.5}, Closed: true,
		}}})
	default:
		return []byte("{}"), nil // graphics/status/commands: accept and ignore
	}
}

func main() {
	out := flag.String("out", "/tmp/cam.nc", "G-code output file")
	postName := flag.String("post", "linuxcnc", "post processor: linuxcnc | grbl")
	op := flag.String("op", "drill", "operation: drill | profile | pocket")
	flag.Parse()
	if err := run(*out, *postName, *op); err != nil {
		fmt.Fprintln(os.Stderr, "camrun:", err)
		os.Exit(1)
	}
}

// run executes the selected job through the real engine against the plate host and writes
// the G-code.
func run(out, postName, op string) error {
	eng := bridge.NewEngine(plateHost{}).SetPost(postName)
	res, err := runOp(eng, op)
	if err != nil {
		return fmt.Errorf("run %s job: %w", op, err)
	}
	if err := os.WriteFile(out, []byte(res.GCode), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("%s: %d G-code lines (%s) → %s\n", op, res.GCodeLines, postName, out)
	return nil
}

// runOp dispatches to the engine's job for the named operation.
func runOp(eng *bridge.Engine, op string) (*bridge.JobResult, error) {
	switch op {
	case "drill":
		return eng.RunDrillingJobOnHost(0)
	case "profile":
		return eng.RunProfileJobOnHost(0)
	case "pocket":
		return eng.RunPocketJobOnHost(0)
	default:
		return nil, fmt.Errorf("unknown op %q (want drill | profile | pocket)", op)
	}
}
