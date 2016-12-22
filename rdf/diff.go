package rdf

import (
	"sort"
	"sync"

	"github.com/google/badwolf/triple"
	"github.com/google/badwolf/triple/node"
	"github.com/google/badwolf/triple/predicate"
)

type Differ interface {
	Run(*node.Node, *Graph, *Graph) (*Diff, error)
}

type Diff struct {
	once sync.Once

	graph           *Graph
	triples         []*triple.Triple
	deletedTriples  map[string]*triple.Triple
	insertedTriples map[string]*triple.Triple
}

func (d *Diff) FullGraph() *Graph {
	return d.graph
}

func (d *Diff) TriplesInDiff() []*triple.Triple {
	d.once.Do(d.setTriplesInDiff)
	return d.triples
}

func (d *Diff) HasDiff() bool {
	d.once.Do(d.setTriplesInDiff)
	return len(d.triples) > 0
}

func NewEmptyDiffFromGraph(g *Graph) *Diff {
	d := Diff{
		graph:           g,
		deletedTriples:  make(map[string]*triple.Triple),
		insertedTriples: make(map[string]*triple.Triple),
	}
	return &d
}

func (d *Diff) AddDeleted(t *triple.Triple, existencePredicate *predicate.Predicate) {
	uuid := t.UUID()
	if d.graph.HasTriple(t) {
		d.deletedTriples[uuid.String()] = t
		if t.Predicate().String() == existencePredicate.String() {
			attachLiteralToTriple(d.graph, t, DiffPredicate, MissingLiteral)
		}
	}
}

func (d *Diff) AddInserted(t *triple.Triple, existencePredicate *predicate.Predicate) {
	uuid := t.UUID()
	if !d.graph.HasTriple(t) {
		d.insertedTriples[uuid.String()] = t
		d.graph.Add(t)
		if t.Predicate().String() == existencePredicate.String() {
			attachLiteralToTriple(d.graph, t, DiffPredicate, ExtraLiteral)
		}
	}
}

func (d *Diff) Deleted() []*triple.Triple {
	var res []*triple.Triple
	for _, v := range d.deletedTriples {
		res = append(res, v)
	}
	sort.Sort(&tripleSorter{res})
	return res
}

func (d *Diff) Inserted() []*triple.Triple {
	var res []*triple.Triple
	for _, v := range d.insertedTriples {
		res = append(res, v)
	}
	sort.Sort(&tripleSorter{res})
	return res
}

func (d *Diff) HasDeletedTriple(t *triple.Triple) bool {
	_, ok := d.deletedTriples[t.UUID().String()]
	return ok
}

func (d *Diff) HasInsertedTriple(t *triple.Triple) bool {
	_, ok := d.insertedTriples[t.UUID().String()]
	return ok
}

func (d *Diff) setTriplesInDiff() {
	var err error
	d.triples, err = d.graph.TriplesForPredicateName("diff")
	if err != nil {
		panic(err)
	}

	sort.Sort(&tripleSorter{d.triples})
}

var DefaultDiffer Differ

type defaultDiffer struct {
	predicate *predicate.Predicate
}

func (d *defaultDiffer) Run(root *node.Node, local *Graph, remote *Graph) (*Diff, error) {
	diff := &Diff{
		graph:           NewGraph(),
		deletedTriples:  make(map[string]*triple.Triple),
		insertedTriples: make(map[string]*triple.Triple),
	}

	remoteT, err := remote.allTriples()
	if err != nil {
		return diff, err
	}
	diff.graph.Add(remoteT...)

	extras, missings, _, err := compareTriplesOf(local, remote)
	if err != nil {
		return diff, err
	}

	for _, extra := range extras {
		diff.AddInserted(extra, d.predicate)
	}

	for _, missing := range missings {
		diff.AddDeleted(missing, d.predicate)
	}

	return diff, nil
}

func compareTriplesOf(localGraph *Graph, remoteGraph *Graph) ([]*triple.Triple, []*triple.Triple, []*triple.Triple, error) {
	var extras, missings, commons []*triple.Triple

	locals, err := localGraph.allTriples()
	if err != nil {
		return extras, missings, commons, err
	}

	remotes, err := remoteGraph.allTriples()
	if err != nil {
		return extras, missings, commons, err
	}

	extras = append(extras, substractTriples(locals, remotes)...)
	missings = append(missings, substractTriples(remotes, locals)...)
	commons = append(commons, intersectTriples(locals, remotes)...)

	return extras, missings, commons, nil
}