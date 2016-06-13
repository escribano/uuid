package uuid

import (
	"crypto/rand"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"sync"
)

var (
	nodeBytes = []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb, 0x44, 0xcc}
)

func TestGenerator_V1(t *testing.T) {
	u := generator.NewV1()

	assert.Equal(t, One, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
}

func TestGenerator_V2(t *testing.T) {
	u := generator.NewV2(DomainGroup)

	assert.Equal(t, Two, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
	assert.Equal(t, uint8(DomainGroup), u.Bytes()[9], "Expected string representation to be valid")

	u = generator.NewV2(DomainUser)

	assert.Equal(t, Two, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
	assert.Equal(t, uint8(DomainUser), u.Bytes()[9], "Expected string representation to be valid")
}

type save struct {
	saved bool
	store *Store
	err   error
	sync.Mutex
}

func (o *save) Save(pStore Store) {
	o.Lock()
	defer o.Unlock()
	o.saved = true
}

func (o *save) Read() (error, Store) {
	if o.store != nil {
		return nil, *o.store
	}
	if o.err != nil {
		return o.err, Store{}
	}
	return nil, Store{}
}

func TestRegisterSaver(t *testing.T) {
	registerTestGenerator(Timestamp(2048), []byte{0xaa})

	saver := &save{store: &Store{}}
	RegisterSaver(saver)

	assert.NotNil(t, generator.Saver, "Saver should save")
	registerDefaultGenerator()
}

func TestSaverRead(t *testing.T) {
	now, node := registerTestGenerator(Now().Sub(time.Second), []byte{0xaa})

	storageStamp := registerSaver(now.Sub(time.Second*2), node)

	assert.NotNil(t, generator.Saver, "Saver should save")
	assert.NotNil(t, generator.Store, "Default generator store should not return an empty store")
	assert.Equal(t, Sequence(2), generator.Store.Sequence, "Successfull read should have actual given sequence")
	assert.True(t, generator.Store.Timestamp > storageStamp, "Failed read should generate a time")
	assert.NotEmpty(t, generator.Store.Node, "There should be a node id")

	// Read returns an error
	_, node = registerTestGenerator(Now(), []byte{0xaa})
	saver := &save{err: errors.New("Read broken")}
	RegisterSaver(saver)

	assert.Nil(t, generator.Saver, "Saver should not exist")
	assert.NotNil(t, generator.Store, "Default generator store should not return an empty store")
	assert.NotEqual(t, Sequence(0), generator.Sequence, "Failed read should generate a non zero random sequence")
	assert.True(t, generator.Timestamp > 0, "Failed read should generate a time")
	assert.Equal(t, node, generator.Node, "There should be a node id")
	registerDefaultGenerator()
}

func TestSaverSave(t *testing.T) {
	registerTestGenerator(Now().Add(1024), nodeBytes)

	saver := &save{}
	RegisterSaver(saver)

	NewV1()

	saver.Lock()
	defer saver.Unlock()

	assert.True(t, saver.saved, "Saver should save")
	registerDefaultGenerator()
}

func TestGeneratorInit(t *testing.T) {
	// A new time that is older than stored time should cause the sequence to increment
	now, node := registerTestGenerator(Now(), nodeBytes)
	storageStamp := registerSaver(now.Add(time.Second), node)

	assert.NotNil(t, generator.Store, "Generator should not return an empty store")
	assert.True(t, generator.Timestamp < storageStamp, "Increment sequence when old timestamp newer than new")
	assert.Equal(t, Sequence(3), generator.Sequence, "Successfull read should have incremented sequence")

	// Nodes not the same should generate a random sequence
	now, node = registerTestGenerator(Now(), nodeBytes)
	storageStamp = registerSaver(now.Sub(time.Second), []byte{0xaa, 0xee, 0xaa, 0xbb, 0x44, 0xcc})

	assert.NotNil(t, generator.Store, "Generator should not return an empty store")
	assert.True(t, generator.Timestamp > storageStamp, "New timestamp should be newer than old")
	assert.NotEqual(t, Sequence(2), generator.Sequence, "Sequence should not be same as storage")
	assert.NotEqual(t, Sequence(3), generator.Sequence, "Sequence should not be incremented but be random")
	assert.Equal(t, generator.Node, node, generator.Sequence, "Node should be equal")

	now, node = registerTestGenerator(Now(), nodeBytes)

	// Random read error should alert user
	generator.Random = func(b []byte) (int, error) {
		return 0, errors.New("EOF")
	}

	storageStamp = registerSaver(now.Sub(time.Second), []byte{0xaa, 0xee, 0xaa, 0xbb, 0x44, 0xcc})

	assert.Error(t, generator.err, "Read error should exist")

	now, node = registerTestGenerator(Now(), nil)
	// Random read error should alert user
	generator.Random = func(b []byte) (int, error) {
		return 0, errors.New("EOF")
	}

	storageStamp = registerSaver(now.Sub(time.Second), []byte{0xaa, 0xee, 0xaa, 0xbb, 0x44, 0xcc})

	assert.Error(t, generator.Error(), "Read error should exist")

	registerDefaultGenerator()
}

func TestGeneratorRead(t *testing.T) {
	// A new time that is older than stored time should cause the sequence to increment
	now := Now()
	i := 0

	timestamps := []Timestamp{
		now.Sub(time.Second),
		now.Sub(time.Second * 2),
	}

	generator = NewGenerator(
		rand.Read,
		func() Timestamp {
			return timestamps[i]
		},
		func() Node {
			return nodeBytes
		})

	storageStamp := registerSaver(now.Add(time.Second), nodeBytes)

	i++

	generator.read()

	assert.True(t, generator.Timestamp != 0, "Should not return an empty store")
	assert.True(t, generator.Timestamp != 0, "Should not return an empty store")
	assert.NotEmpty(t, generator.Node, "Should not return an empty store")

	assert.True(t, generator.Timestamp < storageStamp, "Increment sequence when old timestamp newer than new")
	assert.Equal(t, Sequence(4), generator.Sequence, "Successfull read should have incremented sequence")

	// A new time that is older than stored time should cause the sequence to increment
	now, node := registerTestGenerator(Now().Sub(time.Second), nodeBytes)
	storageStamp = registerSaver(now.Add(time.Second), node)

	generator.read()

	assert.NotEqual(t, 0, generator.Sequence, "Should return an empty store")
	assert.NotEmpty(t, generator.Node, "Should not return an empty store")

	// A new time that is older than stored time should cause the sequence to increment
	registerTestGenerator(Now().Sub(time.Second), nil)
	storageStamp = registerSaver(now.Add(time.Second), []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb})

	generator.read()

	assert.NotEmpty(t, generator.Store, "Should not return an empty store")
	assert.NotEqual(t, []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb}, generator.Node, "Should not return an empty store")

	registerDefaultGenerator()

}

func TestGeneratorRandom(t *testing.T) {
	registerTestGenerator(Now(), []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb})

	b := make([]byte, 6)
	n, err := generator.Random(b)

	assert.NoError(t, err, "There should No be an error", err)
	assert.NotEmpty(t, b, "There should be random data in the slice")
	assert.Equal(t, 6, n, "Amount read should be same as length")

	generator.Random = func(b []byte) (int, error) {
		for i := 0; i < len(b); i++ {
			b[i] = byte(i)
		}
		return len(b), nil
	}

	b = make([]byte, 6)
	n, err = generator.Random(b)
	assert.NoError(t, err, "There should No be an error", err)
	assert.NotEmpty(t, b, "There should be random data in the slice")
	assert.Equal(t, 6, n, "Amount read should be same as length")

	generator.Random = func(b []byte) (int, error) {
		return 0, errors.New("EOF")
	}

	b = make([]byte, 6)
	c := []byte{}
	c = append(c, b...)

	n, err = generator.Random(b)
	assert.Error(t, err, "There should be an error", err)
	assert.Equal(t, 0, n, "Amount read should be same as length")
	assert.Equal(t, c, b, "Slice should be empty")

	id := NewV4()
	assert.Nil(t, id, "There should be no id")
	assert.Error(t, generator.err, "There should be an error [%s]", err)

	registerDefaultGenerator()

}

func TestGeneratorSave(t *testing.T) {
	registerTestGenerator(Now(), []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb})
	generator.read()
	generator.save()
	registerDefaultGenerator()
}

func TestStore_String(t *testing.T) {
	store := &Store{Node: []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb}, Sequence: 2, Timestamp: 3}
	assert.Equal(t, "Timestamp[2167-05-04 23:34:33.709551916 +0000 UTC]-Sequence[2]-Node[ddeeffaabb]", store.String(), "The output store string should match")
}

func TestGetHardwareAddress(t *testing.T) {
	addr := findFirstHardwareAddress()
	assert.NotEmpty(t, addr, "There should be a node id")
}

func registerTestGenerator(pNow Timestamp, pId Node) (Timestamp, Node) {
	generator = NewGenerator(
		rand.Read,
		func() Timestamp {
			return pNow
		},
		func() Node {
			return pId
		})
	return pNow, pId
}

func registerSaver(pStorageStamp Timestamp, pNode Node) (storageStamp Timestamp) {
	storageStamp = pStorageStamp

	saver := &save{store: &Store{Node: pNode, Sequence: 2, Timestamp: pStorageStamp}}
	RegisterSaver(saver)
	return
}
