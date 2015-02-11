// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package httprequest_test

import (
	"io/ioutil"

	"github.com/juju/httprequest"

	gc "gopkg.in/check.v1"
	"gopkg.in/errgo.v1"
)

type marshalSuite struct{}

var _ = gc.Suite(&marshalSuite{})

var marshalTests = []struct {
	about           string
	urlString       string
	method          string
	val             func() interface{}
	expectURLString string
	expectBody      string
	expectError     string
}{{
	about:     "struct with simple fields",
	urlString: "http://localhost:8081/:F1",
	val: func() interface{} {
		t := struct {
			F1 int    `httprequest:",path"`
			F2 string `httprequest:",form"`
		}{
			F1: 99,
			F2: "some text",
		}
		return &t
	},
	expectURLString: "http://localhost:8081/99?F2=some+text",
}, {
	about:     "struct with renamed fields",
	urlString: "http://localhost:8081/:name",
	val: func() interface{} {
		t := struct {
			F1 string `httprequest:"name,path"`
			F2 int    `httprequest:"age,form"`
		}{
			F1: "some random user",
			F2: 42,
		}
		return &t
	},
	expectURLString: "http://localhost:8081/some%20random%20user?age=42",
}, {
	about:     "fields without httprequest tags are ignored",
	urlString: "http://localhost:8081/:name",
	val: func() interface{} {
		t := struct {
			F1 string `httprequest:"name,path"`
			F2 int    `httprequest:"age,form"`
			F3 string
		}{
			F1: "some random user",
			F2: 42,
			F3: "some more random text",
		}
		return &t
	},
	expectURLString: "http://localhost:8081/some%20random%20user?age=42",
}, {
	about:     "pointer fields are correctly handled",
	urlString: "http://localhost:8081/:name",
	val: func() interface{} {
		t := struct {
			F1 *string `httprequest:"name,path"`
			F2 *string `httprequest:"age,form"`
			F3 *string `httprequest:"address,form"`
		}{
			F1: newString("some random user"),
			F2: newString("42"),
		}
		return &t
	},
	expectURLString: "http://localhost:8081/some%20random%20user?age=42",
}, {
	about:     "MarshalText called on TextMarhsalers",
	urlString: "http://localhost:8081/:param1/:param2",
	val: func() interface{} {
		v := struct {
			F1 testMarshaler  `httprequest:"param1,path"`
			F2 *testMarshaler `httprequest:"param2,path"`
			F3 testMarshaler  `httprequest:"param3,form"`
			F4 *testMarshaler `httprequest:"param4,form"`
		}{
			F1: "test1",
			F2: (*testMarshaler)(newString("test2")),
			F3: "test3",
			F4: (*testMarshaler)(newString("test4")),
		}
		return &v
	},
	expectURLString: "http://localhost:8081/test_test1/test_test2?param3=test_test3&param4=test_test4",
}, {
	about:     "MarshalText called on TextMarhsalers",
	urlString: "http://localhost:8081/user/:name:surname",
	val: func() interface{} {
		v := struct {
			F1 testMarshaler  `httprequest:"name,path"`
			F2 *testMarshaler `httprequest:"surname,path"`
		}{
			F1: "name",
			F2: (*testMarshaler)(newString("surname")),
		}
		return &v
	},
	expectURLString: "http://localhost:8081/user/test_nametest_surname",
}, {
	about:     "MarshalText not called on values that do not implement TextMarshaler",
	urlString: "http://localhost:8081/user/:name:surname",
	val: func() interface{} {
		v := struct {
			F1 notTextMarshaler  `httprequest:"name,path"`
			F2 *notTextMarshaler `httprequest:"surname,path"`
		}{
			F1: "name",
			F2: (*notTextMarshaler)(newString("surname")),
		}
		return &v
	},
	expectURLString: "http://localhost:8081/user/namesurname",
}, {
	about:     "MarshalText returns an error",
	urlString: "http://localhost:8081/user/:name:surname",
	val: func() interface{} {
		v := struct {
			F1 testMarshaler  `httprequest:"name,path"`
			F2 *testMarshaler `httprequest:"surname,path"`
		}{
			F1: "",
			F2: (*testMarshaler)(newString("surname")),
		}
		return &v
	},
	expectError: "cannot marshal field: empty string",
}, {
	about:     "[]string field form value",
	urlString: "http://localhost:8081/user",
	val: func() interface{} {
		v := struct {
			F1 []string `httprequest:"users,form"`
		}{
			F1: []string{"user1", "user2", "user3"},
		}
		return &v
	},
	expectURLString: "http://localhost:8081/user?users=user1&users=user2&users=user3",
}, {
	about:     "nil []string field form value",
	urlString: "http://localhost:8081/user",
	val: func() interface{} {
		v := struct {
			F1 *[]string `httprequest:"users,form"`
		}{
			F1: nil,
		}
		return &v
	},
	expectURLString: "http://localhost:8081/user",
}, {
	about:     "[]string field fails to marshal to path",
	urlString: "http://localhost:8081/user/:users",
	val: func() interface{} {
		v := struct {
			F1 []string `httprequest:"users,path"`
		}{
			F1: []string{"user1", "user2", "user3"},
		}
		return &v
	},
	expectError: ".*invalid target type.*",
}, {
	about:     "more than one field with body tag",
	urlString: "http://localhost:8081/user",
	method:    "POST",
	val: func() interface{} {
		v := struct {
			F1 string `httprequest:"user,body"`
			F2 int    `httprequest:"age,body"`
		}{
			F1: "test user",
			F2: 42,
		}
		return &v
	},
	expectError: ".*more than one body field specified",
}, {
	about:     "required path parameter, but not specified",
	urlString: "http://localhost:8081/u/:username",
	method:    "POST",
	val: func() interface{} {
		v := struct {
			F1 string `httprequest:"user,body"`
		}{
			F1: "test user",
		}
		return &v
	},
	expectError: "missing value for path parameter \"username\"",
}, {
	about:     "marshal to body",
	urlString: "http://localhost:8081/u",
	method:    "POST",
	val: func() interface{} {
		type embedded struct {
			F1 string `json:"name"`
			F2 int    `json:"age"`
			F3 string `json:"address"`
		}
		v1 := embedded{
			F1: "test user",
			F2: 42,
			F3: "test address",
		}
		v := struct {
			F1 embedded `httprequest:"info,body"`
		}{
			F1: v1,
		}
		return &v
	},
	expectBody: "{\"name\":\"test user\",\"age\":42,\"address\":\"test address\"}",
}, {
	about:     "empty path wildcard",
	urlString: "http://localhost:8081/u/:",
	method:    "POST",
	val: func() interface{} {
		v := struct {
			F1 string `httprequest:"user,body"`
		}{
			F1: "test user",
		}
		return &v
	},
	expectError: "request wildcards must be named with a non-empty name",
}, {
	about:     "nil field to form",
	urlString: "http://localhost:8081/u",
	val: func() interface{} {
		v := struct {
			F1 *string `httprequest:"user,form"`
		}{}
		return &v
	},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "nil field to path",
	urlString: "http://localhost:8081/u",
	val: func() interface{} {
		v := struct {
			F1 *string `httprequest:"user,path"`
		}{}
		return &v
	},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "marshal to body of a GET request",
	urlString: "http://localhost:8081/u",
	val: func() interface{} {
		v := struct {
			F1 string `httprequest:"user,body"`
		}{
			F1: "hello test",
		}
		return &v
	},
	expectError: "cannot marshal field: trying to marshal to body of a request with method \"GET\"",
}, {
	about:     "marshal to nil value to body",
	urlString: "http://localhost:8081/u",
	val: func() interface{} {
		v := struct {
			F1 *string `httprequest:"user,body"`
		}{
			F1: nil,
		}
		return &v
	},
	expectBody: "",
}, {
	about:     "nil TextMarhsaler",
	urlString: "http://localhost:8081/u",
	val: func() interface{} {
		v := struct {
			F1 *testMarshaler `httprequest:"surname,form"`
		}{
			F1: (*testMarshaler)(nil),
		}
		return &v
	},
	expectURLString: "http://localhost:8081/u",
}, {
	about:     "marshal nil with Sprint",
	urlString: "http://localhost:8081/u",
	val: func() interface{} {
		v := struct {
			F1 *int `httprequest:"surname,form"`
		}{
			F1: (*int)(nil),
		}
		return &v
	},
	expectURLString: "http://localhost:8081/u",
},
}

func (*marshalSuite) TestMarshal(c *gc.C) {
	for i, test := range marshalTests {
		c.Logf("%d: %s", i, test.about)
		method := "GET"
		if test.method != "" {
			method = test.method
		}
		req, err := httprequest.Marshal(test.urlString, method, test.val())
		if test.expectError != "" {
			c.Assert(err, gc.ErrorMatches, test.expectError)
			continue
		}
		c.Assert(err, gc.IsNil)
		if test.expectURLString != "" {
			c.Assert(req.URL.String(), gc.DeepEquals, test.expectURLString)
		}
		//if test.expectBody != "" {
		data, err := ioutil.ReadAll(req.Body)
		c.Assert(err, gc.IsNil)
		c.Assert(string(data), gc.DeepEquals, test.expectBody)
		//}
	}
}

type testMarshaler string

func (t *testMarshaler) MarshalText() ([]byte, error) {
	if len(*t) == 0 {
		return nil, errgo.New("empty string")
	}
	return []byte("test_" + *t), nil
}

type notTextMarshaler string

// MarshalText does *not* implement encoding.TextMarshaler
func (t *notTextMarshaler) MarshalText() {
	panic("unexpected call")
}