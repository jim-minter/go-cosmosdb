// Code generated by github.com/jim-minter/go-cosmosdb, DO NOT EDIT.

package cosmosdb

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/ugorji/go/codec"

	pkg "github.com/jim-minter/go-cosmosdb/example/types"
)

type FakePersonTrigger func(context.Context, *pkg.Person) error
type FakePersonQuery func(PersonClient, *Query) PersonRawIterator

var _ PersonClient = &FakePersonClient{}

func NewFakePersonClient(h *codec.JsonHandle) *FakePersonClient {
	return &FakePersonClient{
		docs:       make(map[string][]byte),
		triggers:   make(map[string]FakePersonTrigger),
		queries:    make(map[string]FakePersonQuery),
		jsonHandle: h,
		lock:       &sync.RWMutex{},
	}
}

type FakePersonClient struct {
	docs       map[string][]byte
	jsonHandle *codec.JsonHandle
	lock       *sync.RWMutex
	triggers   map[string]FakePersonTrigger
	queries    map[string]FakePersonQuery
}

func decodePerson(s []byte, handle *codec.JsonHandle) (*pkg.Person, error) {
	res := &pkg.Person{}
	err := codec.NewDecoder(bytes.NewBuffer(s), handle).Decode(&res)
	return res, err
}

func encodePerson(doc *pkg.Person, handle *codec.JsonHandle) (res []byte, err error) {
	buf := &bytes.Buffer{}
	err = codec.NewEncoder(buf, handle).Encode(doc)
	if err != nil {
		return
	}
	res = buf.Bytes()
	return
}

func (c *FakePersonClient) encodeAndCopy(doc *pkg.Person) (*pkg.Person, []byte, error) {
	encoded, err := encodePerson(doc, c.jsonHandle)
	if err != nil {
		return nil, nil, err
	}
	res, err := decodePerson(encoded, c.jsonHandle)
	if err != nil {
		return nil, nil, err
	}
	return res, encoded, err
}

func (c *FakePersonClient) Create(ctx context.Context, partitionkey string, doc *pkg.Person, options *Options) (*pkg.Person, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, ext := c.docs[doc.ID]
	if ext {
		return nil, &Error{
			StatusCode: http.StatusPreconditionFailed,
			Message:    "Entity with the specified id already exists in the system",
		}
	}

	if options != nil {
		err := c.processPreTriggers(ctx, doc, options)
		if err != nil {
			return nil, err
		}
	}

	res, enc, err := c.encodeAndCopy(doc)
	if err != nil {
		return nil, err
	}
	c.docs[doc.ID] = enc
	return res, nil
}

func (c *FakePersonClient) List(*Options) PersonIterator {
	c.lock.RLock()
	defer c.lock.RUnlock()

	docs := make([]*pkg.Person, 0, len(c.docs))
	for _, d := range c.docs {
		r, err := decodePerson(d, c.jsonHandle)
		if err != nil {
			// todo: ??? what do we do here
			fmt.Print(err)
		}
		docs = append(docs, r)
	}
	return NewFakePersonClientRawIterator(docs)
}

func (c *FakePersonClient) ListAll(context.Context, *Options) (*pkg.People, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	people := &pkg.People{
		Count:     len(c.docs),
		People: make([]*pkg.Person, 0, len(c.docs)),
	}

	for _, d := range c.docs {
		dec, err := decodePerson(d, c.jsonHandle)
		if err != nil {
			return nil, err
		}
		people.People = append(people.People, dec)
	}
	return people, nil
}

func (c *FakePersonClient) Get(ctx context.Context, partitionkey string, documentId string, options *Options) (*pkg.Person, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	out, ext := c.docs[documentId]
	if !ext {
		return nil, &Error{StatusCode: http.StatusNotFound}
	}
	return decodePerson(out, c.jsonHandle)
}

func (c *FakePersonClient) Replace(ctx context.Context, partitionkey string, doc *pkg.Person, options *Options) (*pkg.Person, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, exists := c.docs[doc.ID]
	if !exists {
		return nil, &Error{StatusCode: http.StatusNotFound}
	}

	if options != nil {
		err := c.processPreTriggers(ctx, doc, options)
		if err != nil {
			return nil, err
		}
	}

	res, enc, err := c.encodeAndCopy(doc)
	if err != nil {
		return nil, err
	}
	c.docs[doc.ID] = enc
	return res, nil
}

func (c *FakePersonClient) Delete(ctx context.Context, partitionKey string, doc *pkg.Person, options *Options) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, ext := c.docs[doc.ID]
	if !ext {
		return &Error{StatusCode: http.StatusNotFound}
	}

	delete(c.docs, doc.ID)
	return nil
}

func (c *FakePersonClient) ChangeFeed(*Options) PersonIterator {
	return &fakePersonNotImplementedIterator{}
}

func (c *FakePersonClient) processPreTriggers(ctx context.Context, doc *pkg.Person, options *Options) error {
	for _, trigger := range options.PreTriggers {
		trig, ok := c.triggers[trigger]
		if ok {
			err := trig(ctx, doc)
			if err != nil {
				return err
			}
		} else {
			return ErrNotImplemented
		}
	}
	return nil
}

func (c *FakePersonClient) Query(name string, query *Query, options *Options) PersonRawIterator {
	c.lock.RLock()
	defer c.lock.RUnlock()

	quer, ok := c.queries[query.Query]
	if ok {
		return quer(c, query)
	} else {
		return &fakePersonNotImplementedIterator{}
	}
}

func (c *FakePersonClient) QueryAll(ctx context.Context, partitionkey string, query *Query, options *Options) (*pkg.People, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	quer, ok := c.queries[query.Query]
	if ok {
		items := quer(c, query)
		return items.Next(ctx, -1)
	} else {
		return nil, ErrNotImplemented
	}
}

func (c *FakePersonClient) InjectTrigger(trigger string, impl FakePersonTrigger) {
	c.triggers[trigger] = impl
}

func (c *FakePersonClient) InjectQuery(query string, impl FakePersonQuery) {
	c.queries[query] = impl
}

// NewFakePersonClientRawIterator creates a RawIterator that will produce only
// People from Next() and NextRaw().
func NewFakePersonClientRawIterator(docs []*pkg.Person) PersonRawIterator {
	return &fakePersonClientRawIterator{docs: docs}
}

type fakePersonClientRawIterator struct {
	docs         []*pkg.Person
	continuation int
}

func (i *fakePersonClientRawIterator) Next(ctx context.Context, maxItemCount int) (*pkg.People, error) {
	out := &pkg.People{}
	err := i.NextRaw(ctx, maxItemCount, out)
	return out, err
}

func (i *fakePersonClientRawIterator) NextRaw(ctx context.Context, maxItemCount int, out interface{}) error {
	if i.continuation >= len(i.docs) {
		return nil
	}

	var docs []*pkg.Person
	if maxItemCount == -1 {
		docs = i.docs[i.continuation:]
		i.continuation = len(i.docs)
	} else {
		docs = i.docs[i.continuation : i.continuation+maxItemCount]
		i.continuation += maxItemCount
	}

	d := out.(*pkg.People)
	d.People = docs
	d.Count = len(d.People)
	return nil
}

func (i *fakePersonClientRawIterator) Continuation() string {
	return ""
}

// fakePersonNotImplementedIterator is a RawIterator that will return an error on use.
type fakePersonNotImplementedIterator struct {
}

func (i *fakePersonNotImplementedIterator) Next(ctx context.Context, maxItemCount int) (*pkg.People, error) {
	return nil, ErrNotImplemented
}

func (i *fakePersonNotImplementedIterator) NextRaw(context.Context, int, interface{}) error {
	return ErrNotImplemented
}

func (i *fakePersonNotImplementedIterator) Continuation() string {
	return ""
}
