package plural

import (
	"fmt"
	"testing"
)

func testVars(test *testing.T, value interface{}, expected_f, expected_i int64, expected_n float64, expected_v int, expected_t int64, expected_w int) {
	f, i, n, v, t, w := finvtw(value)
	if expected_f != f || expected_i != i || expected_n != n || expected_v != v || expected_t != t || expected_w != w {
		test.Errorf("`%v` :", value)
		if expected_f != f {
			test.Errorf("\texpected f = %d but got %d", expected_f, f)
		}
		if expected_i != i {
			test.Errorf("\texpected i = %d but got %d", expected_i, i)
		}
		if expected_n != n {
			test.Errorf("\texpected n = %v but got %v", expected_n, n)
		}
		if expected_v != v {
			test.Errorf("\texpected v = %d but got %d", expected_v, v)
		}
		if expected_t != t {
			test.Errorf("\texpected t = %d but got %d", expected_t, t)
		}
		if expected_w != w {
			test.Errorf("\texpected w = %d but got %d", expected_w, w)
		}
	} else if testing.Verbose() {
		fmt.Printf("- Got expected results for <%v>\n", value)
	}
}

func TestPluralVars(t *testing.T) {
	testVars(t, -1, 0, -1, 1.0, 0, 0, 0)
	testVars(t, 1, 0, 1, 1.0, 0, 0, 0)
	testVars(t, "1.0", 0, 1, 1.0, 1, 0, 0)
	testVars(t, "1.00", 0, 1, 1.0, 2, 0, 0)
	testVars(t, 10.20, 2, 10, 10.2, 1, 2, 1)
	testVars(t, "10.20", 20, 10, 10.2, 2, 2, 1)
	testVars(t, -123, 0, -123, 123.0, 0, 0, 0)
	testVars(t, -123.990, 99, -123, 123.99, 2, 99, 2)
	testVars(t, "-123.990", 990, -123, 123.99, 3, 99, 2)
	testVars(t, 0.7+0.1, 8, 0, 0.8, 1, 8, 1)
	testVars(t, 123456.305, 305, 123456, 123456.305, 3, 305, 3)
	testVars(t, 123456.3057892, 3057892, 123456, 123456.3057892, 7, 3057892, 7)
	testVars(t, 123456.3057000, 3057, 123456, 123456.3057, 4, 3057, 4)
	testVars(t, "123456.3057000", 3057000, 123456, 123456.3057, 7, 3057, 4)
	testVars(t, 1000000000000, 0, 1000000000000, 1000000000000, 0, 0, 0)
	testVars(t, 0.33333, 33333, 0, 0.33333, 5, 33333, 5)
}
