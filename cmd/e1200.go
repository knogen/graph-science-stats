/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"graph-science-stats/internal/graph"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// e1200Cmd represents the e1200 command
var e1200Cmd = &cobra.Command{
	Use:   "e1200",
	Short: "计算 E-1200 的熵值",
	Long: `计算 E-1200 的熵值
	不对 linksOut 的年份进行验证,可以确定有穿越的 linksOut`,
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("mode").Value.String() == "1" {
			fmt.Println("计算 E-1200 的小世界熵")
			miniEntropy()
		} else if cmd.Flag("mode").Value.String() == "2" {
			fmt.Println("计算 E-1200 的在 MAG 中的世界熵")
		}
	},
}

func init() {
	rootCmd.AddCommand(e1200Cmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// e1200Cmd.PersistentFlags().String("foo", "aaabbb", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// e1200Cmd.PersistentFlags()
	// e1200Cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	e1200Cmd.Flags().StringP("mode", "m", "1", "1: 计算小世界的 E-1200 熵.\n2: 计算 MAG 中的局部 E-1200 熵.")
}

// comment
// 229:/home/ni/data/wiki/wikiref_data/all_refed_fos_authors_parse.txt  [1965,2010]区间年份的文章数应该就是120万左右

type nodeBasic struct {
	Year int     `bson:"year"`
	Out  []int64 `bson:"out"`
}

type dumpStructEntropy struct {
	InE              []float64
	OutE             []float64
	UndirectedE      []float64
	InSE             []float64
	OutSE            []float64
	UndirectedSE     []float64
	InLength         []int
	OutLength        []int
	UndirectedLength []int
}

type dumpDegreeEntropy struct {
	InE         []float64
	OutE        []float64
	UndirectedE []float64
}

func getMongoCollection() (Client *mongo.Client, collection *mongo.Collection, ctx context.Context) {
	Client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://192.168.1.222:27017"))
	ctx = context.Background()
	err = Client.Connect(ctx)
	if err != nil {
		log.Panic("mongo connect fatil")
	}
	collection = Client.Database("mag2020").Collection("pageinfo_all")
	return
}

func miniEntropy() {
	cache_e1200 := make(map[int64]nodeBasic)
	filePath := "/home/ni/data/wiki/wikiref_data/all_refed_fos_authors_parse.txt"
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewReader(f)
	i := 0
	for {
		i += 1
		line, err := scanner.ReadString('\n')

		if err == io.EOF {
			break
		}
		ss := strings.Split(string(line), "\t")

		var DestIDs []int64
		err = json.Unmarshal([]byte(ss[2]), &DestIDs)
		if err != nil {
			// log.Println(err, filePath, string(line))
		}
		ID, err := strconv.ParseInt(ss[0], 10, 64)
		// 需要对点边进行过滤，保留时间范围内的点
		year, err := strconv.ParseInt(ss[1], 10, 64)
		cache_e1200[ID] = nodeBasic{int(year), DestIDs}
		// for _, destID := range DestIDs {
		// 	edgeChan <- graph.Edge{S: ID, D: destID}
		// }
		// fmt.Println(ID, year, item)
		// break
	}
	f.Close()

	// 过滤大世界，所有的linsOut 都取自大世界
	Client, collection, ctx := getMongoCollection()
	big_e1200 := make(map[int64]nodeBasic)
	for k, _ := range cache_e1200 {
		var bigItem nodeBasic
		result := collection.FindOne(ctx, bson.M{"_id": k}, options.FindOne().SetProjection(bson.M{"out": 1, "year": 1}))
		if err = result.Decode(&bigItem); err != nil {
			log.Fatalln(err, k)
		}
		big_e1200[k] = bigItem
	}
	Client.Disconnect(ctx)
	fmt.Println("bigWorld", len(big_e1200))

	// 过滤 min world, 确保linsout 都是在系统内的
	min_e1200 := make(map[int64]nodeBasic)
	for k, item := range cache_e1200 {
		var newOutArray []int64
		for _, ID := range item.Out {
			if _, ok := cache_e1200[ID]; ok {
				newOutArray = append(newOutArray, ID)
			}
		}
		min_e1200[k] = nodeBasic{item.Year, newOutArray}
	}
	fmt.Println("miniWorld", len(min_e1200))

	var dumpMiniDegreeEntropyObject = dumpDegreeEntropy{}
	var dumpMiniStructEntropyObject = dumpStructEntropy{}

	for year := 1950; year <= 2022; year++ {

		edgeChan := make(chan graph.Edge, 1000)
		var graphNodeRetChan = make(chan []*graph.NodeLink)
		go graph.EdgesToGraphByChan(edgeChan, graphNodeRetChan)
		for k, item := range min_e1200 {
			if item.Year <= year {
				for _, outID := range item.Out {
					edgeChan <- graph.Edge{S: k, D: outID}
				}
			}
		}
		close(edgeChan)
		graphNodeDetail := <-graphNodeRetChan
		fmt.Println("min_e1200", year, len(graphNodeDetail))
		gp := graph.GraphProcess{Node: graphNodeDetail}
		retA := gp.GetDegreeEntropy()

		dumpMiniDegreeEntropyObject.InE = append(dumpMiniDegreeEntropyObject.InE, retA.InE)
		dumpMiniDegreeEntropyObject.OutE = append(dumpMiniDegreeEntropyObject.OutE, retA.OutE)
		dumpMiniDegreeEntropyObject.UndirectedE = append(dumpMiniDegreeEntropyObject.UndirectedE, retA.UndirectedE)

		retB := gp.GetStructEntropy()
		dumpMiniStructEntropyObject.InE = append(dumpMiniStructEntropyObject.InE, retB.InE)
		dumpMiniStructEntropyObject.OutE = append(dumpMiniStructEntropyObject.OutE, retB.OutE)
		dumpMiniStructEntropyObject.UndirectedE = append(dumpMiniStructEntropyObject.UndirectedE, retB.UndirectedE)
		dumpMiniStructEntropyObject.InSE = append(dumpMiniStructEntropyObject.InSE, retB.InSE)
		dumpMiniStructEntropyObject.OutSE = append(dumpMiniStructEntropyObject.OutSE, retB.OutSE)
		dumpMiniStructEntropyObject.UndirectedSE = append(dumpMiniStructEntropyObject.UndirectedSE, retB.UndirectedSE)
		dumpMiniStructEntropyObject.InLength = append(dumpMiniStructEntropyObject.InLength, retB.InLength)
		dumpMiniStructEntropyObject.OutLength = append(dumpMiniStructEntropyObject.OutLength, retB.OutLength)
		dumpMiniStructEntropyObject.UndirectedLength = append(dumpMiniStructEntropyObject.UndirectedLength, retB.UndirectedLength)

	}

	var dumpHugeDegreeEntropyObject = dumpDegreeEntropy{}
	var dumpHugeStructEntropyObject = dumpStructEntropy{}

	for year := 1950; year <= 2022; year++ {

		edgeChan := make(chan graph.Edge, 1000)
		var graphNodeRetChan = make(chan []*graph.NodeLink)
		go graph.EdgesToGraphByChan(edgeChan, graphNodeRetChan)
		for k, item := range big_e1200 {
			if item.Year <= year {
				for _, outID := range item.Out {
					edgeChan <- graph.Edge{S: k, D: outID}
				}
			}
		}
		close(edgeChan)
		graphNodeDetail := <-graphNodeRetChan
		fmt.Println("big_e1200", year, len(graphNodeDetail))
		gp := graph.GraphProcess{Node: graphNodeDetail}
		retA := gp.GetDegreeEntropy()

		dumpHugeDegreeEntropyObject.InE = append(dumpHugeDegreeEntropyObject.InE, retA.InE)
		dumpHugeDegreeEntropyObject.OutE = append(dumpHugeDegreeEntropyObject.OutE, retA.OutE)
		dumpHugeDegreeEntropyObject.UndirectedE = append(dumpHugeDegreeEntropyObject.UndirectedE, retA.UndirectedE)

		retB := gp.GetStructEntropy()
		dumpHugeStructEntropyObject.InE = append(dumpHugeStructEntropyObject.InE, retB.InE)
		dumpHugeStructEntropyObject.OutE = append(dumpHugeStructEntropyObject.OutE, retB.OutE)
		dumpHugeStructEntropyObject.UndirectedE = append(dumpHugeStructEntropyObject.UndirectedE, retB.UndirectedE)
		dumpHugeStructEntropyObject.InSE = append(dumpHugeStructEntropyObject.InSE, retB.InSE)
		dumpHugeStructEntropyObject.OutSE = append(dumpHugeStructEntropyObject.OutSE, retB.OutSE)
		dumpHugeStructEntropyObject.UndirectedSE = append(dumpHugeStructEntropyObject.UndirectedSE, retB.UndirectedSE)
		dumpHugeStructEntropyObject.InLength = append(dumpHugeStructEntropyObject.InLength, retB.InLength)
		dumpHugeStructEntropyObject.OutLength = append(dumpHugeStructEntropyObject.OutLength, retB.OutLength)
		dumpHugeStructEntropyObject.UndirectedLength = append(dumpHugeStructEntropyObject.UndirectedLength, retB.UndirectedLength)

	}

	file, _ := json.MarshalIndent(dumpMiniDegreeEntropyObject, "", " ")
	_ = ioutil.WriteFile("/tmp/dumpMiniDegreeEntropyObject.json", file, 0644)

	file, _ = json.MarshalIndent(dumpMiniStructEntropyObject, "", " ")
	_ = ioutil.WriteFile("/tmp/dumpMiniStructEntropyObject.json", file, 0644)

	file, _ = json.MarshalIndent(dumpHugeDegreeEntropyObject, "", " ")
	_ = ioutil.WriteFile("/tmp/dumpHugeDegreeEntropyObject.json", file, 0644)

	file, _ = json.MarshalIndent(dumpHugeStructEntropyObject, "", " ")
	_ = ioutil.WriteFile("/tmp/dumpHugeStructEntropyObject.json", file, 0644)

	// fmt.Println(gp.GetDegreeEntropy())
	// fmt.Println(gp.GetStructEntropy())
	// close(edgeChan)

}
