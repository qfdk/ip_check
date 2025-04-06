package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

// IPResponse 最终返回的JSON结构
type IPResponse struct {
	IP       string `json:"ip"`
	Location struct {
		Country   string `json:"country"`
		Region    string `json:"region"`
		City      string `json:"city"`
		ISP       string `json:"isp,omitempty"`
		Latitude  float64 `json:"latitude,omitempty"`
		Longitude float64 `json:"longitude,omitempty"`
	} `json:"location"`
}

var db *geoip2.Reader

func main() {
	var err error
	// 加载MaxMind数据库
	db, err = geoip2.Open("./GeoLite2-City.mmdb")
	if err != nil {
		log.Printf("加载MaxMind数据库失败: %v", err)
		log.Fatal(err)
	}
	defer db.Close()
	log.Println("成功加载MaxMind数据库")

	http.HandleFunc("/", handleIP)
	http.HandleFunc("/json", handleIPJSON)
	
	port := 8088
	fmt.Printf("IP检测服务运行在 http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handleIP(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)
	
	acceptHeader := r.Header.Get("Accept")
	formatParam := r.URL.Query().Get("format")
	
	if strings.Contains(acceptHeader, "application/json") || formatParam == "json" {
		response := getIPResponse(ip)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, ip+"\n")
	}
}

func handleIPJSON(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)
	response := getIPResponse(ip)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getIPResponse(ip string) IPResponse {
	response := IPResponse{IP: ip}
	
	parsedIP := net.ParseIP(ip)
	if parsedIP != nil && db != nil {
		// 查询城市信息
		city, err := db.City(parsedIP)
		if err == nil {
			if len(city.Country.Names["zh-CN"]) > 0 {
				response.Location.Country = city.Country.Names["zh-CN"]
			} else {
				response.Location.Country = city.Country.Names["en"]
			}
			
			if len(city.Subdivisions) > 0 {
				if len(city.Subdivisions[0].Names["zh-CN"]) > 0 {
					response.Location.Region = city.Subdivisions[0].Names["zh-CN"]
				} else {
					response.Location.Region = city.Subdivisions[0].Names["en"]
				}
			}
			
			if len(city.City.Names["zh-CN"]) > 0 {
				response.Location.City = city.City.Names["zh-CN"]
			} else {
				response.Location.City = city.City.Names["en"]
			}
			
			response.Location.Latitude = city.Location.Latitude
			response.Location.Longitude = city.Location.Longitude
		}
		
		// 可选：查询ISP信息（需要ISP数据库）
		isp, err := db.ISP(parsedIP)
		if err == nil {
      response.Location.ISP = isp.ISP
		}
	}
	
	return response
}

func getClientIP(r *http.Request) string {
	headers := []string{
		"CF-Connecting-IP",
		"True-Client-IP",
		"X-Real-IP",
		"X-Client-IP",
		"Fastly-Client-IP",
		"X-Forwarded-For",
	}
	
	for _, header := range headers {
		if value := r.Header.Get(header); value != "" {
			if header == "X-Forwarded-For" {
				ips := strings.Split(value, ",")
				if len(ips) > 0 {
					ip := strings.TrimSpace(ips[0])
					if validIP := validateIP(ip); validIP != "" {
						return validIP
					}
				}
			} else {
				if validIP := validateIP(value); validIP != "" {
					return validIP
				}
			}
		}
	}
	
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if validIP := validateIP(remoteIP); validIP != "" {
			return validIP
		}
	} else {
		if validIP := validateIP(r.RemoteAddr); validIP != "" {
			return validIP
		}
	}
	
	return r.RemoteAddr
}

func validateIP(ipStr string) string {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return ""
	}
	return ip.String()
}
