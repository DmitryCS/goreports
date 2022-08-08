package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"goreports/config"
	"image"
	"image/color"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"gocv.io/x/gocv"
)

func getStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got /status request\n")
	io.WriteString(w, "Status check passed!\n")
}

type HeatmapsJSON struct {
    Heatmap_array []uint32 `json:"heatmap_array"`
    Width int `json:"width"`
	Height int  `json:"height"`
	Final_width int `json:"final_width"` 
    Final_height int `json:"final_height"`
    Transparency float32 `json:"transparency"`
}

func toBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

type Triple struct {
    IndexI int32
	IndexJ int32
    Value int32
}
type ByKey []Triple
func (s ByKey) Len() int {
    return len(s)
}

func (s ByKey) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}

func (s ByKey) Less(i, j int) bool {
    return s[i].Value < s[j].Value
}

func findFinalPoints(matrix_compressed gocv.Mat, mx int32) [][]float32{
	h, w := int32(matrix_compressed.Size()[0]), int32(matrix_compressed.Size()[1])
	triples := []Triple{}
	for i:=0;i<int(h);i++{
		for j:=0;j<int(w);j++{
			val := int32(matrix_compressed.GetFloatAt(i, j))
			if val > mx / 2{
				triples = append(triples, Triple{IndexI: int32(i), IndexJ: int32(j), Value: val})		
			}
		}
	}
	sort.Sort(ByKey(triples))
	bool_matrix := make([][]bool, h)
	for i := range bool_matrix {
		bool_matrix[i] = make([]bool, w)
	}
	radius := int32(15)
	final_points := [][]float32{}
	for _, triple := range triples{
		if len(final_points) > 15{
			break
		}
		if bool_matrix[triple.IndexI][triple.IndexJ]{
			continue
		}
		y_begin, x_begin := triple.IndexI - radius, triple.IndexJ - radius 		
		y_end, x_end := triple.IndexI + radius, triple.IndexJ + radius 		
		if y_begin < 0{
			y_begin = 0
		}
		if x_begin < 0{
			x_begin = 0
		}
		if y_end > h{
			y_end = h
		}
		if x_end > w{
			x_end = w
		}
		for y:=y_begin;y<y_end;y++{
			for x:=x_begin;x<x_end;x++{
				if (x - triple.IndexJ) * (x - triple.IndexJ) + (y - triple.IndexI) *(y - triple.IndexI) <= radius *radius{
					bool_matrix[y][x] = true
				}				
			}
		}
		final_points = append(final_points, []float32{float32(triple.Value), float32(triple.IndexJ) / float32(w), float32(triple.IndexI) / float32(h)})
	}
	return final_points
}

func postHeatmaps(w http.ResponseWriter, r *http.Request) {
	time_start_request := time.Now().UnixNano()   
	// Load data in Mat.gocv
	decoder := json.NewDecoder(r.Body)
    var data_json HeatmapsJSON
    err := decoder.Decode(&data_json)
    if err != nil {
        panic(err)
    }	  
	h1, w1 := data_json.Height, data_json.Width
	num_cores := runtime.NumCPU()
	step:= int(float32(h1*w1)/float32(num_cores))
	var wg sync.WaitGroup
	wg.Add(num_cores)
	byte_matrix := make([][]byte, num_cores)
	for i,core_ind:=0,0; i<len(data_json.Heatmap_array); i,core_ind=i+step, core_ind+1{
		WorkOnConvertToByteArray(&wg, data_json.Heatmap_array[i:i+step], byte_matrix, core_ind)
	}
	byte_array := []byte{}
	for i:=0; i<num_cores; i++{
		byte_array = append(byte_array, byte_matrix[i]...)
	}
	heatmapI32, _ := gocv.NewMatWithSizesFromBytes([]int{h1, w1}, gocv.MatTypeCV32SC1, byte_array)
	heatmapF32 := gocv.NewMat()
	heatmapI32.ConvertTo(&heatmapF32, gocv.MatTypeCV32F)

	// find max points on heatmap 
	_, max_valf, _, _ := gocv.MinMaxIdx(heatmapF32)
	max_val := int32(max_valf)
	const CORE int = 3
	compressed_h, compressed_w := int(h1 / CORE), int(w1 / CORE)
	matrix_compressed := gocv.NewMat()
	gocv.Resize(heatmapF32, &matrix_compressed, image.Pt(compressed_w, compressed_h), 0, 0, gocv.InterpolationNearestNeighbor)
	final_points := findFinalPoints(matrix_compressed, max_val)

	// make heatmap image from matrix
	img_out := gocv.NewMat()
	heatmapF32.MultiplyFloat(float32(255)/float32(max_val))
	heatmapU8 := gocv.NewMat()
	heatmapF32.ConvertTo(&heatmapU8, gocv.MatTypeCV8UC1)
	gocv.ApplyColorMap(heatmapU8, &img_out, gocv.ColormapJet)

	// change transparency with 0 detects to 0 and other to data_json.Transparency
	matrix128 := gocv.NewMatWithSize(h1, w1, gocv.MatTypeCV8UC1)
	matrix128.AddUChar(128)
	matrix0 := gocv.NewMatWithSize(h1, w1, gocv.MatTypeCV8UC1)
	transparency := uint8(255*(1.0-data_json.Transparency))
	matrixTransparency := gocv.NewMatWithSize(h1, w1, gocv.MatTypeCV8UC1)	
	matrixTransparency.AddUChar(transparency)
	matrix255 := gocv.NewMatWithSize(h1, w1, gocv.MatTypeCV8UC1)
	matrix255.AddUChar(255)

	splited := gocv.Split(img_out)
	dst1 := gocv.NewMat()
	dst2 := gocv.NewMat()
	dst3 := gocv.NewMat()
	gocv.Compare(splited[0], matrix128, &dst1, gocv.CompareEQ)
	gocv.Compare(splited[1], matrix0, &dst2, gocv.CompareEQ)
	gocv.Compare(splited[2], matrix0, &dst3, gocv.CompareEQ)
	dst4 := gocv.NewMat()
	gocv.BitwiseAnd(dst1, dst2, &dst4)
	dst5 := gocv.NewMat()
	gocv.BitwiseAnd(dst3, dst4, &dst5)
	dst6 := gocv.NewMat()
	gocv.BitwiseXor(matrix255, dst5, &dst6)
	dst7 := gocv.NewMat()
	gocv.BitwiseAnd(matrixTransparency, dst6, &dst7)
	img_out4 := gocv.NewMat()
	splited = append(splited, dst7)
	gocv.Merge(splited, &img_out4)

	// resize to needed shape and draw max points on heatmap 
	col := color.RGBA{255, 255, 255, 255}
	resizeMat := gocv.NewMat()
	gocv.Resize(img_out4, &resizeMat, image.Pt(data_json.Final_width, data_json.Final_height), 0, 0, gocv.InterpolationArea)
	for _, val := range final_points{
		x := int(val[1]*float32(data_json.Final_width))
		y := int(val[2]*float32(data_json.Final_height))
		p := image.Pt(x-20, y+10)
		gocv.PutText(&resizeMat, strconv.Itoa(int(float32(val[0]) / float32(CORE * CORE)))+"s", p, gocv.FontHersheySimplex, 0.8, col, 2) 
	}

	// encode gocv image to png 
	buf, _ := gocv.IMEncode(".png", resizeMat) 
	bytes := buf.GetBytes() 
	var base64Encoding string		
	base64Encoding += toBase64(bytes)
	log.Println("Full request time: ", float64(time.Now().UnixNano() - time_start_request)/float64(1e9))
	io.WriteString(w, base64Encoding)
}

func WorkOnConvertToByteArray(wg *sync.WaitGroup, heatmap_array []uint32, byte_array [][]byte, core_ind int){
	local_byte_array := []byte{}
	for _, val := range heatmap_array{
		local_byte_array = append(local_byte_array, toByteArray(val)...)
	}
	byte_array[core_ind] = local_byte_array
	wg.Done()
}

func toByteArray(val uint32) ([]byte) {
	arr := make([]byte, 4)
	binary.LittleEndian.PutUint32(arr[0:4], val)
    return arr
}

func main() {
	fmt.Println(config.HttpServerConfig.URL)
	http.HandleFunc("/status", getStatus)
	http.HandleFunc("/heatmaps", postHeatmaps)
	err := http.ListenAndServe(":" + strconv.Itoa(config.HttpServerConfig.PORT), nil)
	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}
