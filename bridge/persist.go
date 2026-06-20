// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"fmt"

	"oblikovati.org/api/types"
)

// CAMAttributeSet is the document attribute-set namespace the CAM add-in persists into (the
// add-in id); CAMJobAttribute holds the serialised job and CAMToolsAttribute the tool library.
const (
	CAMAttributeSet   = "com.oblikovati.cam"
	CAMJobAttribute   = "job"
	CAMToolsAttribute = "tools"
)

// SaveToolLibrary persists the tool library into the active document (best-effort: it is a
// no-op when no document is open, since the library is also held in the session).
func (e *Engine) SaveToolLibrary() error {
	docID, err := e.activeDocumentID()
	if err != nil {
		return nil //nolint:nilerr // no document yet: keep the session library only
	}
	e.mu.Lock()
	lib := e.library
	e.mu.Unlock()
	payload, err := json.Marshal(lib)
	if err != nil {
		return fmt.Errorf("marshal tool library: %w", err)
	}
	_, err = e.api.Attributes().Set(docID, CAMAttributeSet, CAMToolsAttribute, types.StringVariant(string(payload)))
	return err
}

// LoadToolLibrary reads the tool library stored in the active document into the engine, leaving
// the current library unchanged when none is stored.
func (e *Engine) LoadToolLibrary() error {
	docID, err := e.activeDocumentID()
	if err != nil {
		return nil //nolint:nilerr // no document: keep the default library
	}
	res, err := e.api.Attributes().Get(docID, CAMAttributeSet, CAMToolsAttribute)
	if err != nil || !res.Found {
		return nil //nolint:nilerr // nothing stored: keep the default library
	}
	payload, ok := res.Attribute.Value.Str()
	if !ok {
		return fmt.Errorf("CAM tool-library attribute is not a string value")
	}
	var lib ToolLibrary
	if err := json.Unmarshal([]byte(payload), &lib); err != nil {
		return fmt.Errorf("unmarshal tool library: %w", err)
	}
	e.mu.Lock()
	e.library = lib
	e.mu.Unlock()
	return nil
}

// jobDoc is the serialisable form of a Job: the tools, post config, and the operation
// configurations (parameters only — the driving geometry is re-resolved from the part on
// load, mirroring how a recompute re-reads selections). Operations are a tagged union via
// opDoc.Kind.
type jobDoc struct {
	GeometryTolerance float64          `json:"geometryTolerance"`
	PostProcessor     string           `json:"postProcessor"`
	Tools             []ToolController `json:"tools"`
	Operations        []opDoc          `json:"operations"`
}

// opDoc is the tagged, flattened serialisation of one operation. Only the fields relevant to
// Kind are populated; the rest stay at their zero value (omitempty keeps the JSON compact).
type opDoc struct {
	Kind            string  `json:"kind"`
	Label           string  `json:"label"`
	Active          bool    `json:"active"`
	ToolController  int     `json:"toolController"`
	ClearanceHeight float64 `json:"clearanceHeight,omitempty"`
	SafeHeight      float64 `json:"safeHeight,omitempty"`
	RetractHeight   float64 `json:"retractHeight,omitempty"`
	StartDepth      float64 `json:"startDepth,omitempty"`
	FinalDepth      float64 `json:"finalDepth,omitempty"`
	Coolant         string  `json:"coolant,omitempty"`

	// Drilling
	DwellTime   float64 `json:"dwellTime,omitempty"`
	PeckDepth   float64 `json:"peckDepth,omitempty"`
	ChipBreak   bool    `json:"chipBreak,omitempty"`
	FeedRetract bool    `json:"feedRetract,omitempty"`
	Repeat      int     `json:"repeat,omitempty"`

	// Profile / Engrave / Pocket / MillFace
	Side        string  `json:"side,omitempty"`
	OffsetExtra float64 `json:"offsetExtra,omitempty"`
	Climb       bool    `json:"climb,omitempty"`
	StepDown    float64 `json:"stepDown,omitempty"`
	StepOver    float64 `json:"stepOver,omitempty"`

	// Helix
	HoleRadius float64 `json:"holeRadius,omitempty"`
	Pitch      float64 `json:"pitch,omitempty"`
	Direction  string  `json:"direction,omitempty"`

	// Thread mill
	MajorDiameter float64 `json:"majorDiameter,omitempty"`
	Internal      bool    `json:"internal,omitempty"`

	// Surface / Waterline (3D finishing) — geometry is re-resolved from the part mesh, not persisted.
	Sampling float64 `json:"sampling,omitempty"`
	Zigzag   bool    `json:"zigzag,omitempty"`

	// Rest
	PrevToolDiameter float64 `json:"prevToolDiameter,omitempty"`

	// Chamfer
	Width     float64 `json:"width,omitempty"`
	ToolAngle float64 `json:"toolAngle,omitempty"`

	// Trochoidal
	LoopRadius float64 `json:"loopRadius,omitempty"`
	Advance    float64 `json:"advance,omitempty"`

	// Probe
	ProbeFeed float64 `json:"probeFeed,omitempty"`

	Dressups []dressupDoc `json:"dressups,omitempty"`
}

// dressupDoc is the tagged serialisation of one toolpath dressup; only the fields for Kind
// are populated.
type dressupDoc struct {
	Kind     string  `json:"kind"`
	Count    int     `json:"count,omitempty"`
	Width    float64 `json:"width,omitempty"`
	Height   float64 `json:"height,omitempty"`
	Style    string  `json:"style,omitempty"`
	Length   float64 `json:"length,omitempty"`
	MinAngle float64 `json:"minAngle,omitempty"`
	Side     string  `json:"side,omitempty"`
}

// MarshalJob serialises a job's configuration to JSON (excluding resolved geometry).
func MarshalJob(job *Job) (string, error) {
	doc := jobDoc{GeometryTolerance: job.GeometryTolerance, PostProcessor: job.PostProcessor, Tools: job.Tools}
	for _, op := range job.Operations {
		d, err := toOpDoc(op)
		if err != nil {
			return "", err
		}
		doc.Operations = append(doc.Operations, d)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal job: %w", err)
	}
	return string(b), nil
}

// UnmarshalJob rebuilds a job's configuration from JSON. The reconstructed operations carry
// their parameters but no geometry (Holes/Boundary) — the engine re-resolves that from the
// part before running them.
func UnmarshalJob(s string) (*Job, error) {
	var doc jobDoc
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	job := NewJob()
	job.GeometryTolerance = doc.GeometryTolerance
	job.PostProcessor = doc.PostProcessor
	job.Tools = doc.Tools
	for _, d := range doc.Operations {
		op, err := fromOpDoc(d)
		if err != nil {
			return nil, err
		}
		job.Operations = append(job.Operations, op)
	}
	return job, nil
}

// baseDoc copies an operation's common envelope fields (including its dressup chain) into an
// opDoc.
func baseDoc(kind string, b OpBase) opDoc {
	return opDoc{
		Kind: kind, Label: b.OpLabel, Active: b.IsActive, ToolController: b.ToolController,
		ClearanceHeight: b.ClearanceHeight, SafeHeight: b.SafeHeight, RetractHeight: b.RetractHeight,
		StartDepth: b.StartDepth, FinalDepth: b.FinalDepth, Coolant: b.Coolant, Dressups: dressupDocs(b.Dressups),
	}
}

// opBaseFrom rebuilds the common envelope (including dressups) from an opDoc.
func opBaseFrom(d opDoc) OpBase {
	return OpBase{
		OpLabel: d.Label, IsActive: d.Active, ToolController: d.ToolController,
		ClearanceHeight: d.ClearanceHeight, SafeHeight: d.SafeHeight, RetractHeight: d.RetractHeight,
		StartDepth: d.StartDepth, FinalDepth: d.FinalDepth, Coolant: d.Coolant, Dressups: dressupsFrom(d.Dressups),
	}
}

// dressupDocs serialises a dressup chain to its tagged form.
func dressupDocs(ds []Dressup) []dressupDoc {
	var out []dressupDoc
	for _, d := range ds {
		switch x := d.(type) {
		case TagsDressup:
			out = append(out, dressupDoc{Kind: "tags", Count: x.Params.Count, Width: x.Params.Width, Height: x.Params.Height})
		case DogboneDressup:
			out = append(out, dressupDoc{Kind: "dogbone", Style: x.Params.Style, Length: x.Params.Length, MinAngle: x.Params.MinAngle, Side: x.Params.Side})
		case RampDressup:
			out = append(out, dressupDoc{Kind: "ramp", Length: x.Params.Length, MinAngle: x.Params.Angle})
		case LeadInOutDressup:
			out = append(out, dressupDoc{Kind: "leadinout", Length: x.Params.Radius, Side: x.Params.Side})
		}
	}
	return out
}

// dressupsFrom rebuilds a dressup chain from its tagged serialisation, skipping unknown kinds.
func dressupsFrom(docs []dressupDoc) []Dressup {
	var out []Dressup
	for _, d := range docs {
		switch d.Kind {
		case "tags":
			out = append(out, NewTagsDressup(d.Count, d.Width, d.Height))
		case "dogbone":
			out = append(out, NewDogboneDressup(d.Style, d.Length, d.MinAngle, d.Side))
		case "ramp":
			out = append(out, NewRampDressup(d.Length, d.MinAngle))
		case "leadinout":
			out = append(out, NewLeadInOutDressup(d.Length, d.Side))
		}
	}
	return out
}

// toOpDoc converts an operation to its tagged serialisation.
func toOpDoc(op Operation) (opDoc, error) {
	switch o := op.(type) {
	case *DrillingOp:
		d := baseDoc("drilling", o.OpBase)
		d.DwellTime, d.PeckDepth, d.ChipBreak, d.FeedRetract, d.Repeat = o.DwellTime, o.PeckDepth, o.ChipBreak, o.FeedRetract, o.Repeat
		return d, nil
	case *ProfileOp:
		d := baseDoc("profile", o.OpBase)
		d.Side, d.OffsetExtra, d.Climb, d.StepDown = o.Side, o.OffsetExtra, o.Climb, o.StepDown
		return d, nil
	case *PocketOp:
		d := baseDoc("pocket", o.OpBase)
		d.StepOver, d.Climb, d.StepDown = o.StepOver, o.Climb, o.StepDown
		return d, nil
	case *AdaptiveOp:
		d := baseDoc("adaptive", o.OpBase)
		d.StepOver, d.Climb, d.StepDown = o.StepOver, o.Climb, o.StepDown
		return d, nil
	case *RestOp:
		d := baseDoc("rest", o.OpBase)
		d.PrevToolDiameter, d.StepOver, d.Climb, d.StepDown = o.PrevToolDiameter, o.StepOver, o.Climb, o.StepDown
		return d, nil
	case *TrochoidalOp:
		d := baseDoc("trochoidal", o.OpBase)
		d.LoopRadius, d.Advance, d.Side, d.StepDown = o.LoopRadius, o.Advance, o.Side, o.StepDown
		return d, nil
	case *MillFaceOp:
		d := baseDoc("millface", o.OpBase)
		d.StepOver, d.StepDown = o.StepOver, o.StepDown
		return d, nil
	case *EngraveOp:
		d := baseDoc("engrave", o.OpBase)
		d.Climb, d.StepDown = o.Climb, o.StepDown
		return d, nil
	case *ChamferOp:
		d := baseDoc("chamfer", o.OpBase)
		d.Width, d.ToolAngle, d.Side, d.Climb = o.Width, o.ToolAngle, o.Side, o.Climb
		return d, nil
	case *SlotOp:
		d := baseDoc("slot", o.OpBase)
		d.Width, d.StepOver, d.Climb, d.StepDown = o.Width, o.StepOver, o.Climb, o.StepDown
		return d, nil
	case *ProbeOp:
		d := baseDoc("probe", o.OpBase)
		d.ProbeFeed = o.ProbeFeed
		return d, nil
	case *HelixOp:
		d := baseDoc("helix", o.OpBase)
		d.HoleRadius, d.Pitch, d.Direction = o.HoleRadius, o.Pitch, o.Direction
		return d, nil
	case *ThreadMillOp:
		d := baseDoc("threadmill", o.OpBase)
		d.MajorDiameter, d.Pitch, d.Internal, d.Climb = o.MajorDiameter, o.Pitch, o.Internal, o.Climb
		return d, nil
	case *SurfaceOp:
		d := baseDoc("surface", o.OpBase)
		d.StepOver, d.Sampling, d.Zigzag = o.StepOver, o.Sampling, o.Zigzag
		return d, nil
	case *WaterlineOp:
		d := baseDoc("waterline", o.OpBase)
		d.StepOver, d.StepDown = o.StepOver, o.StepDown
		return d, nil
	default:
		return opDoc{}, fmt.Errorf("cannot serialise operation of type %T", op)
	}
}

// fromOpDoc reconstructs an operation (without geometry) from its tagged serialisation.
func fromOpDoc(d opDoc) (Operation, error) {
	switch d.Kind {
	case "drilling":
		return &DrillingOp{OpBase: opBaseFrom(d), DwellTime: d.DwellTime, PeckDepth: d.PeckDepth, ChipBreak: d.ChipBreak, FeedRetract: d.FeedRetract, Repeat: d.Repeat}, nil
	case "profile":
		return &ProfileOp{OpBase: opBaseFrom(d), Side: d.Side, OffsetExtra: d.OffsetExtra, Climb: d.Climb, StepDown: d.StepDown}, nil
	case "pocket":
		return &PocketOp{OpBase: opBaseFrom(d), StepOver: d.StepOver, Climb: d.Climb, StepDown: d.StepDown}, nil
	case "adaptive":
		return &AdaptiveOp{OpBase: opBaseFrom(d), StepOver: d.StepOver, Climb: d.Climb, StepDown: d.StepDown}, nil
	case "rest":
		return &RestOp{OpBase: opBaseFrom(d), PrevToolDiameter: d.PrevToolDiameter, StepOver: d.StepOver, Climb: d.Climb, StepDown: d.StepDown}, nil
	case "trochoidal":
		return &TrochoidalOp{OpBase: opBaseFrom(d), LoopRadius: d.LoopRadius, Advance: d.Advance, Side: d.Side, StepDown: d.StepDown}, nil
	case "millface":
		return &MillFaceOp{OpBase: opBaseFrom(d), StepOver: d.StepOver, StepDown: d.StepDown}, nil
	case "engrave":
		return &EngraveOp{OpBase: opBaseFrom(d), Climb: d.Climb, StepDown: d.StepDown}, nil
	case "chamfer":
		return &ChamferOp{OpBase: opBaseFrom(d), Width: d.Width, ToolAngle: d.ToolAngle, Side: d.Side, Climb: d.Climb}, nil
	case "slot":
		return &SlotOp{OpBase: opBaseFrom(d), Width: d.Width, StepOver: d.StepOver, Climb: d.Climb, StepDown: d.StepDown}, nil
	case "probe":
		return &ProbeOp{OpBase: opBaseFrom(d), ProbeFeed: d.ProbeFeed}, nil
	case "helix":
		return &HelixOp{OpBase: opBaseFrom(d), HoleRadius: d.HoleRadius, Pitch: d.Pitch, Direction: d.Direction}, nil
	case "threadmill":
		return &ThreadMillOp{OpBase: opBaseFrom(d), MajorDiameter: d.MajorDiameter, Pitch: d.Pitch, Internal: d.Internal, Climb: d.Climb}, nil
	case "surface":
		return &SurfaceOp{OpBase: opBaseFrom(d), StepOver: d.StepOver, Sampling: d.Sampling, Zigzag: d.Zigzag}, nil
	case "waterline":
		return &WaterlineOp{OpBase: opBaseFrom(d), StepOver: d.StepOver, StepDown: d.StepDown}, nil
	default:
		return nil, fmt.Errorf("unknown operation kind %q", d.Kind)
	}
}

// SaveJob persists a job into the active document's CAM attribute set. The job survives the
// .obk save/load through the host's attribute store.
func (e *Engine) SaveJob(job *Job) error {
	docID, err := e.activeDocumentID()
	if err != nil {
		return err
	}
	payload, err := MarshalJob(job)
	if err != nil {
		return err
	}
	_, err = e.api.Attributes().Set(docID, CAMAttributeSet, CAMJobAttribute, types.StringVariant(payload))
	return err
}

// LoadJob reads the persisted job back from the active document, or (nil, nil) when none is
// stored.
func (e *Engine) LoadJob() (*Job, error) {
	docID, err := e.activeDocumentID()
	if err != nil {
		return nil, err
	}
	res, err := e.api.Attributes().Get(docID, CAMAttributeSet, CAMJobAttribute)
	if err != nil {
		return nil, err
	}
	if !res.Found {
		return nil, nil
	}
	payload, ok := res.Attribute.Value.Str()
	if !ok {
		return nil, fmt.Errorf("CAM job attribute is not a string value")
	}
	return UnmarshalJob(payload)
}

// activeDocumentID returns the id of the active document, erroring when none is open.
func (e *Engine) activeDocumentID() (uint64, error) {
	list, err := e.api.Documents().List()
	if err != nil {
		return 0, err
	}
	for _, d := range list.Documents {
		if d.Active {
			return d.ID, nil
		}
	}
	return 0, fmt.Errorf("no active document to store the CAM job in")
}
