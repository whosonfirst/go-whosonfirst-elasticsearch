package main

import (
	"context"
        "flag"
        "fmt"
	
        "github.com/whosonfirst/go-whosonfirst-elasticsearch/document"
)

func main(){

        flag.Parse()

        for _, raw := range flag.Args(){
                fmt.Println(prepare(raw))
        }
}

//export prepare
func prepare(str_body string) string {

	ctx := context.Background()
	
	doc, err := document.PrepareSpelunkerV1Document(ctx, []byte(str_body))

	if err != nil {
		return err.Error()
	}

	return string(doc)
}
