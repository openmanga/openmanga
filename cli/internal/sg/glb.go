package sg

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Bone is one skeleton joint in bind pose (skeleton.pose()), local to its parent bone.
type Bone struct {
	Name   string     `json:"name"`
	Parent int        `json:"parent"` // index into Bones, -1 = child of the armature
	T      [3]float64 `json:"t"`
	R      [4]float64 `json:"r"` // quaternion x, y, z, w
	S      [3]float64 `json:"s"`
}

// Skeleton is what camera framing needs from a character model.
type Skeleton struct {
	OriginalHeight float64    `json:"originalHeight"`
	BoxMin         [3]float64 `json:"boxMin"` // mesh bounds (model space)
	BoxMax         [3]float64 `json:"boxMax"`
	ArmatureT      [3]float64 `json:"armatureT"`
	ArmatureR      [4]float64 `json:"armatureR"`
	ArmatureS      [3]float64 `json:"armatureS"`
	Bones          []Bone     `json:"bones"`
}

type gltfNode struct {
	Name        string    `json:"name"`
	Children    []int     `json:"children"`
	Translation []float64 `json:"translation"`
	Rotation    []float64 `json:"rotation"`
	Scale       []float64 `json:"scale"`
	Matrix      []float64 `json:"matrix"`
	Mesh        *int      `json:"mesh"`
	Skin        *int      `json:"skin"`
}

type gltfDoc struct {
	Scene  int `json:"scene"`
	Scenes []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []gltfNode `json:"nodes"`
	Skins []struct {
		Joints              []int `json:"joints"`
		InverseBindMatrices *int  `json:"inverseBindMatrices"`
	} `json:"skins"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
		} `json:"primitives"`
	} `json:"meshes"`
	Accessors []struct {
		BufferView    *int   `json:"bufferView"`
		ByteOffset    int    `json:"byteOffset"`
		ComponentType int    `json:"componentType"`
		Count         int    `json:"count"`
		Type          string `json:"type"`
	} `json:"accessors"`
	BufferViews []struct {
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
}

func (n gltfNode) local() Mat4 {
	if len(n.Matrix) == 16 {
		var m Mat4
		copy(m[:], n.Matrix)
		return m
	}
	t := Vec3{}
	if len(n.Translation) == 3 {
		t = Vec3{n.Translation[0], n.Translation[1], n.Translation[2]}
	}
	r := QuatIdentity
	if len(n.Rotation) == 4 {
		r = Quat{n.Rotation[0], n.Rotation[1], n.Rotation[2], n.Rotation[3]}
	}
	s := Vec3{1, 1, 1}
	if len(n.Scale) == 3 {
		s = Vec3{n.Scale[0], n.Scale[1], n.Scale[2]}
	}
	return Compose(t, r, s)
}

// floats reads a float32 accessor as n-component tuples.
func floats(doc *gltfDoc, bin []byte, acc int, comps int) ([][]float64, error) {
	a := doc.Accessors[acc]
	if a.BufferView == nil || a.ComponentType != 5126 {
		return nil, fmt.Errorf("unsupported accessor %d", acc)
	}
	bv := doc.BufferViews[*a.BufferView]
	stride := bv.ByteStride
	if stride == 0 {
		stride = comps * 4
	}
	out := make([][]float64, a.Count)
	for i := 0; i < a.Count; i++ {
		off := bv.ByteOffset + a.ByteOffset + i*stride
		if off+comps*4 > len(bin) {
			return nil, fmt.Errorf("accessor %d out of range", acc)
		}
		v := make([]float64, comps)
		for c := 0; c < comps; c++ {
			v[c] = float64(math.Float32frombits(binary.LittleEndian.Uint32(bin[off+c*4:])))
		}
		out[i] = v
	}
	return out, nil
}

// ParseSkeleton extracts the bind-pose skeleton and bounds of a character .glb
// the way Character.js prepares it (skinned meshes under the scene root, LOD
// meshes from index 1 when there are several).
func ParseSkeleton(path string) (*Skeleton, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 20 || string(data[0:4]) != "glTF" {
		return nil, fmt.Errorf("%s is not a binary glTF (.glb)", path)
	}
	jsonLen := int(binary.LittleEndian.Uint32(data[12:16]))
	var doc gltfDoc
	if err := json.Unmarshal(data[20:20+jsonLen], &doc); err != nil {
		return nil, err
	}
	var bin []byte
	if off := 20 + jsonLen; off+8 <= len(data) {
		binLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		bin = data[off+8 : off+8+binLen]
	}
	parent := make([]int, len(doc.Nodes))
	for i := range parent {
		parent[i] = -1
	}
	for i, n := range doc.Nodes {
		for _, c := range n.Children {
			parent[c] = i
		}
	}
	roots := []int{}
	if len(doc.Scenes) > 0 {
		roots = doc.Scenes[doc.Scene].Nodes
	}
	// skinned meshes: scene root children first, else anywhere (custom models)
	var meshes []int
	var order []int
	var walk func(i int)
	walk = func(i int) {
		order = append(order, i)
		for _, c := range doc.Nodes[i].Children {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	for _, r := range roots {
		if doc.Nodes[r].Mesh != nil && doc.Nodes[r].Skin != nil {
			meshes = append(meshes, r)
		}
	}
	if len(meshes) == 0 {
		for _, i := range order {
			if doc.Nodes[i].Mesh != nil && doc.Nodes[i].Skin != nil {
				meshes = append(meshes, i)
			}
		}
	}
	if len(meshes) == 0 {
		return nil, fmt.Errorf("%s has no skinned mesh", path)
	}
	start := 0
	if len(meshes) > 1 {
		start = 1
	}
	skin := doc.Skins[*doc.Nodes[meshes[start]].Skin]
	isBone := map[int]int{}
	for i, j := range skin.Joints {
		isBone[j] = i
	}
	ibm := make([]Mat4, len(skin.Joints))
	if skin.InverseBindMatrices != nil {
		rows, err := floats(&doc, bin, *skin.InverseBindMatrices, 16)
		if err != nil {
			return nil, err
		}
		for i := range ibm {
			copy(ibm[i][:], rows[i])
		}
	} else {
		for i := range ibm {
			ibm[i] = Identity
		}
	}
	sk := &Skeleton{}
	// armature = parent of the first bone in scene traversal order
	arm := -1
	for _, i := range order {
		if _, ok := isBone[i]; ok {
			arm = parent[i]
			break
		}
	}
	am := Identity
	if arm >= 0 {
		am = doc.Nodes[arm].local()
	}
	t, r, s := am.Decompose()
	sk.ArmatureT, sk.ArmatureR, sk.ArmatureS = [3]float64{t.X, t.Y, t.Z}, [4]float64{r.X, r.Y, r.Z, r.W}, [3]float64{s.X, s.Y, s.Z}
	for i, j := range skin.Joints {
		world := ibm[i].Inverse()
		p := -1
		if pi, ok := isBone[parent[j]]; ok && parent[j] >= 0 {
			p = pi
			world = ibm[pi].Mul(world) // parentWorld^-1 * world
		}
		t, r, s := world.Decompose()
		sk.Bones = append(sk.Bones, Bone{Name: doc.Nodes[j].Name, Parent: p, T: [3]float64{t.X, t.Y, t.Z}, R: [4]float64{r.X, r.Y, r.Z, r.W}, S: [3]float64{s.X, s.Y, s.Z}})
	}
	// bounds of the LOD meshes in their local space (Box3.setFromObject(lod))
	lo := Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi := Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, mi := range meshes[start:] {
		n := doc.Nodes[mi]
		m := n.local()
		for _, prim := range doc.Meshes[*n.Mesh].Primitives {
			pos, ok := prim.Attributes["POSITION"]
			if !ok {
				continue
			}
			pts, err := floats(&doc, bin, pos, 3)
			if err != nil {
				return nil, err
			}
			for _, p := range pts {
				v := m.Apply(Vec3{p[0], p[1], p[2]})
				lo = Vec3{math.Min(lo.X, v.X), math.Min(lo.Y, v.Y), math.Min(lo.Z, v.Z)}
				hi = Vec3{math.Max(hi.X, v.X), math.Max(hi.Y, v.Y), math.Max(hi.Z, v.Z)}
			}
		}
	}
	sk.BoxMin, sk.BoxMax = [3]float64{lo.X, lo.Y, lo.Z}, [3]float64{hi.X, hi.Y, hi.Z}
	sk.OriginalHeight = hi.Y - lo.Y
	return sk, nil
}
