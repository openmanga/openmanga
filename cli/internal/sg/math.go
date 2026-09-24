package sg

import "math"

// Minimal three.js-compatible math: right-handed, Y up, column-major matrices.

type Vec3 struct{ X, Y, Z float64 }

func V(x, y, z float64) Vec3       { return Vec3{x, y, z} }
func (a Vec3) Add(b Vec3) Vec3     { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3     { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Mul(s float64) Vec3  { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64  { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Len() float64        { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Dist(b Vec3) float64 { return a.Sub(b).Len() }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l == 0 {
		return a
	}
	return a.Mul(1 / l)
}
func (a Vec3) SetLen(l float64) Vec3 { return a.Norm().Mul(l) }

type Quat struct{ X, Y, Z, W float64 }

var QuatIdentity = Quat{0, 0, 0, 1}

func (a Quat) Mul(b Quat) Quat {
	return Quat{
		a.X*b.W + a.W*b.X + a.Y*b.Z - a.Z*b.Y,
		a.Y*b.W + a.W*b.Y + a.Z*b.X - a.X*b.Z,
		a.Z*b.W + a.W*b.Z + a.X*b.Y - a.Y*b.X,
		a.W*b.W - a.X*b.X - a.Y*b.Y - a.Z*b.Z,
	}
}

func (q Quat) Norm() Quat {
	l := math.Sqrt(q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W)
	if l == 0 {
		return QuatIdentity
	}
	return Quat{q.X / l, q.Y / l, q.Z / l, q.W / l}
}

func AxisAngle(axis Vec3, angle float64) Quat {
	a := axis.Norm()
	s := math.Sin(angle / 2)
	return Quat{a.X * s, a.Y * s, a.Z * s, math.Cos(angle / 2)}
}

// Rotate applies q to v.
func (q Quat) Rotate(v Vec3) Vec3 {
	ix := q.W*v.X + q.Y*v.Z - q.Z*v.Y
	iy := q.W*v.Y + q.Z*v.X - q.X*v.Z
	iz := q.W*v.Z + q.X*v.Y - q.Y*v.X
	iw := -q.X*v.X - q.Y*v.Y - q.Z*v.Z
	return Vec3{
		ix*q.W + iw*-q.X + iy*-q.Z - iz*-q.Y,
		iy*q.W + iw*-q.Y + iz*-q.X - ix*-q.Z,
		iz*q.W + iw*-q.Z + ix*-q.Y - iy*-q.X,
	}
}

// Euler is three.js Euler with an order ("XYZ" default, "YXZ" for cameras).
type Euler struct {
	X, Y, Z float64
	Order   string
}

// Quat is Quaternion.setFromEuler.
func (e Euler) Quat() Quat {
	c1, c2, c3 := math.Cos(e.X/2), math.Cos(e.Y/2), math.Cos(e.Z/2)
	s1, s2, s3 := math.Sin(e.X/2), math.Sin(e.Y/2), math.Sin(e.Z/2)
	switch e.Order {
	case "YXZ":
		return Quat{s1*c2*c3 + c1*s2*s3, c1*s2*c3 - s1*c2*s3, c1*c2*s3 - s1*s2*c3, c1*c2*c3 + s1*s2*s3}
	default: // XYZ
		return Quat{s1*c2*c3 + c1*s2*s3, c1*s2*c3 - s1*c2*s3, c1*c2*s3 + s1*s2*c3, c1*c2*c3 - s1*s2*s3}
	}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// EulerFromQuat is Euler.setFromQuaternion (via the rotation matrix).
func EulerFromQuat(q Quat, order string) Euler {
	m := Compose(Vec3{}, q, Vec3{1, 1, 1})
	m11, m12, m13 := m[0], m[4], m[8]
	m21, m22, m23 := m[1], m[5], m[9]
	m31, m32, m33 := m[2], m[6], m[10]
	e := Euler{Order: order}
	switch order {
	case "YXZ":
		e.X = math.Asin(-clamp(m23, -1, 1))
		if math.Abs(m23) < 0.9999999 {
			e.Y = math.Atan2(m13, m33)
			e.Z = math.Atan2(m21, m22)
		} else {
			e.Y = math.Atan2(-m31, m11)
		}
	default:
		e.Order = "XYZ"
		e.Y = math.Asin(clamp(m13, -1, 1))
		if math.Abs(m13) < 0.9999999 {
			e.X = math.Atan2(-m23, m33)
			e.Z = math.Atan2(-m12, m11)
		} else {
			e.X = math.Atan2(m32, m22)
		}
	}
	return e
}

// Mat4 is column-major like three.js Matrix4.elements.
type Mat4 [16]float64

var Identity = Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

func (a Mat4) Mul(b Mat4) Mat4 {
	var r Mat4
	for c := 0; c < 4; c++ {
		for row := 0; row < 4; row++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[k*4+row] * b[c*4+k]
			}
			r[c*4+row] = s
		}
	}
	return r
}

func Compose(p Vec3, q Quat, s Vec3) Mat4 {
	x, y, z, w := q.X, q.Y, q.Z, q.W
	x2, y2, z2 := x+x, y+y, z+z
	xx, xy, xz := x*x2, x*y2, x*z2
	yy, yz, zz := y*y2, y*z2, z*z2
	wx, wy, wz := w*x2, w*y2, w*z2
	return Mat4{
		(1 - (yy + zz)) * s.X, (xy + wz) * s.X, (xz - wy) * s.X, 0,
		(xy - wz) * s.Y, (1 - (xx + zz)) * s.Y, (yz + wx) * s.Y, 0,
		(xz + wy) * s.Z, (yz - wx) * s.Z, (1 - (xx + yy)) * s.Z, 0,
		p.X, p.Y, p.Z, 1,
	}
}

func (m Mat4) Pos() Vec3 { return Vec3{m[12], m[13], m[14]} }

func (m Mat4) Apply(v Vec3) Vec3 {
	w := m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]
	if w == 0 {
		w = 1
	}
	return Vec3{
		(m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]) / w,
		(m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]) / w,
		(m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]) / w,
	}
}

// Decompose is Matrix4.decompose.
func (m Mat4) Decompose() (Vec3, Quat, Vec3) {
	sx := Vec3{m[0], m[1], m[2]}.Len()
	sy := Vec3{m[4], m[5], m[6]}.Len()
	sz := Vec3{m[8], m[9], m[10]}.Len()
	if m.Det() < 0 {
		sx = -sx
	}
	r := m
	r[0], r[1], r[2] = r[0]/sx, r[1]/sx, r[2]/sx
	r[4], r[5], r[6] = r[4]/sy, r[5]/sy, r[6]/sy
	r[8], r[9], r[10] = r[8]/sz, r[9]/sz, r[10]/sz
	return m.Pos(), quatFromRotation(r), Vec3{sx, sy, sz}
}

func quatFromRotation(m Mat4) Quat {
	m11, m12, m13 := m[0], m[4], m[8]
	m21, m22, m23 := m[1], m[5], m[9]
	m31, m32, m33 := m[2], m[6], m[10]
	tr := m11 + m22 + m33
	switch {
	case tr > 0:
		s := 0.5 / math.Sqrt(tr+1)
		return Quat{(m32 - m23) * s, (m13 - m31) * s, (m21 - m12) * s, 0.25 / s}
	case m11 > m22 && m11 > m33:
		s := 2 * math.Sqrt(1+m11-m22-m33)
		return Quat{0.25 * s, (m12 + m21) / s, (m13 + m31) / s, (m32 - m23) / s}
	case m22 > m33:
		s := 2 * math.Sqrt(1+m22-m11-m33)
		return Quat{(m12 + m21) / s, 0.25 * s, (m23 + m32) / s, (m13 - m31) / s}
	default:
		s := 2 * math.Sqrt(1+m33-m11-m22)
		return Quat{(m13 + m31) / s, (m23 + m32) / s, 0.25 * s, (m21 - m12) / s}
	}
}

func (m Mat4) Det() float64 {
	n11, n12, n13, n14 := m[0], m[4], m[8], m[12]
	n21, n22, n23, n24 := m[1], m[5], m[9], m[13]
	n31, n32, n33, n34 := m[2], m[6], m[10], m[14]
	n41, n42, n43, n44 := m[3], m[7], m[11], m[15]
	return n41*(n14*n23*n32-n13*n24*n32-n14*n22*n33+n12*n24*n33+n13*n22*n34-n12*n23*n34) +
		n42*(n11*n23*n34-n11*n24*n33+n14*n21*n33-n13*n21*n34+n13*n24*n31-n14*n23*n31) +
		n43*(n11*n24*n32-n11*n22*n34-n14*n21*n32+n12*n21*n34+n14*n22*n31-n12*n24*n31) +
		n44*(-n13*n22*n31-n11*n23*n32+n11*n22*n33+n13*n21*n32-n12*n21*n33+n12*n23*n31)
}

// Inverse is Matrix4.getInverse (zero matrix when singular).
func (m Mat4) Inverse() Mat4 {
	n11, n21, n31, n41 := m[0], m[1], m[2], m[3]
	n12, n22, n32, n42 := m[4], m[5], m[6], m[7]
	n13, n23, n33, n43 := m[8], m[9], m[10], m[11]
	n14, n24, n34, n44 := m[12], m[13], m[14], m[15]
	t11 := n23*n34*n42 - n24*n33*n42 + n24*n32*n43 - n22*n34*n43 - n23*n32*n44 + n22*n33*n44
	t12 := n14*n33*n42 - n13*n34*n42 - n14*n32*n43 + n12*n34*n43 + n13*n32*n44 - n12*n33*n44
	t13 := n13*n24*n42 - n14*n23*n42 + n14*n22*n43 - n12*n24*n43 - n13*n22*n44 + n12*n23*n44
	t14 := n14*n23*n32 - n13*n24*n32 - n14*n22*n33 + n12*n24*n33 + n13*n22*n34 - n12*n23*n34
	det := n11*t11 + n21*t12 + n31*t13 + n41*t14
	if det == 0 {
		return Mat4{}
	}
	d := 1 / det
	var r Mat4
	r[0] = t11 * d
	r[1] = (n24*n33*n41 - n23*n34*n41 - n24*n31*n43 + n21*n34*n43 + n23*n31*n44 - n21*n33*n44) * d
	r[2] = (n22*n34*n41 - n24*n32*n41 + n24*n31*n42 - n21*n34*n42 - n22*n31*n44 + n21*n32*n44) * d
	r[3] = (n23*n32*n41 - n22*n33*n41 - n23*n31*n42 + n21*n33*n42 + n22*n31*n43 - n21*n32*n43) * d
	r[4] = t12 * d
	r[5] = (n13*n34*n41 - n14*n33*n41 + n14*n31*n43 - n11*n34*n43 - n13*n31*n44 + n11*n33*n44) * d
	r[6] = (n14*n32*n41 - n12*n34*n41 - n14*n31*n42 + n11*n34*n42 + n12*n31*n44 - n11*n32*n44) * d
	r[7] = (n12*n33*n41 - n13*n32*n41 + n13*n31*n42 - n11*n33*n42 - n12*n31*n43 + n11*n32*n43) * d
	r[8] = t13 * d
	r[9] = (n14*n23*n41 - n13*n24*n41 - n14*n21*n43 + n11*n24*n43 + n13*n21*n44 - n11*n23*n44) * d
	r[10] = (n12*n24*n41 - n14*n22*n41 + n14*n21*n42 - n11*n24*n42 - n12*n21*n44 + n11*n22*n44) * d
	r[11] = (n13*n22*n41 - n12*n23*n41 - n13*n21*n42 + n11*n23*n42 + n12*n21*n43 - n11*n22*n43) * d
	r[12] = t14 * d
	r[13] = (n13*n24*n31 - n14*n23*n31 + n14*n21*n33 - n11*n24*n33 - n13*n21*n34 + n11*n23*n34) * d
	r[14] = (n14*n22*n31 - n12*n24*n31 - n14*n21*n32 + n11*n24*n32 + n12*n21*n34 - n11*n22*n34) * d
	r[15] = (n12*n23*n31 - n13*n22*n31 + n13*n21*n32 - n11*n23*n32 - n12*n21*n33 + n11*n22*n33) * d
	return r
}

// LookAtQuat is Object3D.lookAt for a camera (-Z faces the target), up = +Y.
func LookAtQuat(eye, target Vec3, camera bool) Quat {
	var z Vec3
	if camera {
		z = eye.Sub(target)
	} else {
		z = target.Sub(eye)
	}
	if z.Len() == 0 {
		z = Vec3{0, 0, 1}
	}
	z = z.Norm()
	up := Vec3{0, 1, 0}
	x := up.Cross(z)
	if x.Len() == 0 {
		z.Z += 0.0001
		z = z.Norm()
		x = up.Cross(z)
	}
	x = x.Norm()
	y := z.Cross(x)
	m := Mat4{x.X, x.Y, x.Z, 0, y.X, y.Y, y.Z, 0, z.X, z.Y, z.Z, 0, 0, 0, 0, 1}
	return quatFromRotation(m)
}

func Deg(r float64) float64 { return r * 180 / math.Pi }
func Rad(d float64) float64 { return d * math.Pi / 180 }
