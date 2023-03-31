package server

import (
	"fastRPC/service"
	"fmt"
	"html/template"
	"net/http"
)

const debugText = `<html>
	<body>
	<title>FastRPC Services</title>
	{{range .}}
	<hr>
	Service <b>{{.Name}}</b>
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mType := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mType.ArgType}}, {{$mType.ReplyType}}) error</td>
			<td align=center>{{$mType.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>`

var debug = template.Must(template.New("RPC debug").Parse(debugText))

type debugHTTP struct {
	*Server
}

type debugService struct {
	Name   string
	Method map[string]*service.MethodType
}

// Runs at /debug/fastrpc
// 在这里，我们将返回一个 HTML 报文，这个报文将展示注册所有的 service 的每一个方法的调用情况
func (server debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Build a sorted version of the data.
	var services []debugService
	server.serviceMap.Range(func(nameItem, svcItem interface{}) bool {
		svc := svcItem.(*service.Service)
		services = append(services, debugService{
			Name:   nameItem.(string),
			Method: svc.GetMethodMap(),
		})
		return true
	})

	err := debug.Execute(w, services)
	if err != nil {
		_, _ = fmt.Fprintln(w, "FastRPC: error executing template:", err.Error())
	}
}
