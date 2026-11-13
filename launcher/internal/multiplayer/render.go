package multiplayer

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
)

type Camera struct {
	Eye, Forward, Up Vec3
	FOV              float64
	At               time.Time
	Calibrated       bool
}
type ScreenPoint struct{ X, Y float64 }
type Color struct{ R, G, B uint8 }
type ScreenTriangle struct {
	Points [3]ScreenPoint
	Color  Color
	Depth  float64
}
type Label struct {
	Point ScreenPoint
	Name  string
	Color Color
}
type DrawList struct {
	Triangles []ScreenTriangle
	Labels    []Label
}

func (c Camera) Project(p Vec3, width, height int) (ScreenPoint, float64, bool) {
	if width <= 0 || height <= 0 || !p.Valid() || !c.Eye.Valid() || !finite(c.FOV) || c.FOV < 10 || c.FOV > 150 {
		return ScreenPoint{}, 0, false
	}
	forward := c.Forward.Unit()
	if forward.Len() < 0.9 {
		return ScreenPoint{}, 0, false
	}
	up := c.Up.Unit()
	if up.Len() < 0.9 {
		up = Vec3{Y: 1}
	}
	right := forward.Cross(up).Unit()
	if right.Len() < 0.9 {
		return ScreenPoint{}, 0, false
	}
	up = right.Cross(forward).Unit()
	relative := p.Sub(c.Eye)
	z := relative.Dot(forward)
	if z < 0.15 {
		return ScreenPoint{}, z, false
	}
	factor := float64(height) / 2 / math.Tan(c.FOV*math.Pi/360)
	return ScreenPoint{float64(width)/2 + relative.Dot(right)*factor/z, float64(height)/2 - relative.Dot(up)*factor/z}, z, true
}
func rotateY(p Vec3, yaw float64) Vec3 {
	s, c := math.Sincos(yaw)
	return Vec3{p.X*c + p.Z*s, p.Y, -p.X*s + p.Z*c}
}
func rotateX(p Vec3, angle float64) Vec3 {
	s, c := math.Sincos(angle)
	return Vec3{p.X, p.Y*c - p.Z*s, p.Y*s + p.Z*c}
}

type triangle struct {
	v     [3]Vec3
	color Color
}

func box(out *[]triangle, pivot, center, half Vec3, angle, yaw float64, origin Vec3, color Color) {
	v := make([]Vec3, 8)
	for i := 0; i < 8; i++ {
		x, y, z := -half.X, -half.Y, -half.Z
		if i&1 != 0 {
			x = half.X
		}
		if i&2 != 0 {
			y = half.Y
		}
		if i&4 != 0 {
			z = half.Z
		}
		p := Vec3{x, y, z}.Add(center)
		p = rotateX(p.Sub(pivot), angle).Add(pivot)
		v[i] = rotateY(p, yaw).Add(origin)
	}
	faces := [][4]int{{0, 2, 3, 1}, {4, 5, 7, 6}, {0, 4, 6, 2}, {1, 3, 7, 5}, {2, 6, 7, 3}, {0, 1, 5, 4}}
	shades := []float64{0.64, 1, 0.77, 0.88, 1.08, 0.55}
	for i, f := range faces {
		shade := func(x uint8) uint8 { return uint8(math.Min(255, float64(x)*shades[i])) }
		c := Color{shade(color.R), shade(color.G), shade(color.B)}
		*out = append(*out, triangle{[3]Vec3{v[f[0]], v[f[1]], v[f[2]]}, c}, triangle{[3]Vec3{v[f[0]], v[f[2]], v[f[3]]}, c})
	}
}
func body(p Player) []triangle {
	color := Color{72, 162, 107}
	if p.Slot == 1 {
		color = Color{76, 145, 218}
	}
	skin := Color{222, 176, 132}
	boots := Color{94, 66, 47}
	hair := Color{116, 83, 46}
	var out []triangle
	add := func(pivot, center, half Vec3, angle float64, c Color) {
		box(&out, pivot, center, half, angle, p.Pose.Yaw, p.Pose.Position, c)
	}
	add(Vec3{}, Vec3{Y: 1.23}, Vec3{.25, .3, .14}, 0, color)
	add(Vec3{}, Vec3{Y: .92}, Vec3{.25, .06, .15}, 0, boots)
	add(Vec3{}, Vec3{Y: 1.67}, Vec3{.155, .19, .15}, 0, skin)
	add(Vec3{}, Vec3{Y: 1.83, Z: -.015}, Vec3{.17, .07, .17}, 0, hair)
	swing := 0.0
	if math.Hypot(p.Pose.Velocity.X, p.Pose.Velocity.Z) > .2 {
		swing = math.Sin(p.Pose.Phase) * .65
	}
	for _, side := range []float64{-1, 1} {
		pivot := Vec3{X: side * .14, Y: .9}
		angle := side * swing
		add(pivot, Vec3{X: side * .14, Y: .53}, Vec3{.105, .36, .105}, angle, Color{185, 172, 135})
		add(pivot, Vec3{X: side * .14, Y: .13, Z: .045}, Vec3{.12, .13, .19}, angle, boots)
		arm := Vec3{X: side * .34, Y: 1.49}
		add(arm, Vec3{X: side * .34, Y: 1.3}, Vec3{.095, .2, .105}, -angle, color)
		add(arm, Vec3{X: side * .34, Y: 1.03}, Vec3{.075, .13, .075}, -angle, skin)
	}
	return out
}

// BuildDrawList draws a procedural avatar in world coordinates. It has no
// access to the game's depth buffer: this prototype has no wall occlusion,
// collisions, native Link mesh or game-side actor authority.
func BuildDrawList(camera Camera, players []Player, width, height int, now time.Time) DrawList {
	var out DrawList
	if camera.At.IsZero() || now.Sub(camera.At) > 1500*time.Millisecond {
		return out
	}
	for _, p := range players {
		if !p.Pose.Ready || !p.Pose.Valid() || p.Pose.Position.Sub(camera.Eye).Len() > 300 {
			continue
		}
		for _, tri := range body(p) {
			t := ScreenTriangle{Color: tri.color}
			valid := true
			for i, v := range tri.v {
				point, z, ok := camera.Project(v, width, height)
				if !ok || math.Abs(point.X) > 100000 || math.Abs(point.Y) > 100000 {
					valid = false
					break
				}
				t.Points[i] = point
				t.Depth += z / 3
			}
			if valid {
				out.Triangles = append(out.Triangles, t)
			}
		}
		point, _, ok := camera.Project(p.Pose.Position.Add(Vec3{Y: 2.1}), width, height)
		if ok && point.X > -200 && point.X < float64(width)+200 && point.Y > -30 && point.Y < float64(height)+30 {
			c := Color{118, 225, 163}
			if p.Slot == 1 {
				c = Color{123, 192, 255}
			}
			out.Labels = append(out.Labels, Label{Point: point, Name: p.Name, Color: c})
		}
	}
	sort.SliceStable(out.Triangles, func(i, j int) bool { return out.Triangles[i].Depth > out.Triangles[j].Depth })
	return out
}

// WriteSVG previews the same geometry on Linux CI; it is not a game screenshot.
func WriteSVG(w io.Writer, list DrawList, width, height int) error {
	if _, e := fmt.Fprintf(w, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\"><rect width=\"100%%\" height=\"100%%\" fill=\"#14202b\"/>", width, height, width, height); e != nil {
		return e
	}
	for _, t := range list.Triangles {
		fmt.Fprintf(w, "<polygon points=\"%.2f,%.2f %.2f,%.2f %.2f,%.2f\" fill=\"#%02x%02x%02x\"/>", t.Points[0].X, t.Points[0].Y, t.Points[1].X, t.Points[1].Y, t.Points[2].X, t.Points[2].Y, t.Color.R, t.Color.G, t.Color.B)
	}
	for _, l := range list.Labels {
		name := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;").Replace(l.Name)
		fmt.Fprintf(w, "<text x=\"%.2f\" y=\"%.2f\" fill=\"#%02x%02x%02x\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"18\">%s</text>", l.Point.X, l.Point.Y, l.Color.R, l.Color.G, l.Color.B, name)
	}
	_, e := io.WriteString(w, "</svg>")
	return e
}
