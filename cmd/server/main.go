// Zadanie 1 - Aplikacja pogodowa (Programowanie Aplikacji w Chmurze Obliczeniowej).
// Autor: Hubert Kolejko.
//
// Aplikacja serwuje prosty formularz HTML pozwalający wybrać kraj i miasto
// z predefiniowanej listy, a następnie wyświetla bieżące dane pogodowe
// pobrane z publicznego API Open-Meteo (bez klucza API).
//
// Brak zewnętrznych zależności - wyłącznie biblioteka standardowa Go,
// dzięki czemu wynikowa binarka jest statyczna i mieści się w obrazie scratch.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// Author wpisywany w logu startowym i w etykiecie OCI.
const Author = "Hubert Kolejko"

// City reprezentuje pojedynczą lokalizację dostępną w formularzu.
// Lat/Lon są wymagane przez Open-Meteo (forecast endpoint).
type City struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// Country reprezentuje kraj wraz z listą miast - używane w szablonie HTML.
type Country struct {
	Code   string
	Name   string
	Cities []City
}

// Predefiniowana lista krajów i miast.
// Współrzędne podane na potrzeby zapytania do Open-Meteo.
var countries = []Country{
	{Code: "PL", Name: "Polska", Cities: []City{
		{"Warszawa", 52.2297, 21.0122},
		{"Kraków", 50.0647, 19.9450},
		{"Gdańsk", 54.3520, 18.6466},
		{"Wrocław", 51.1079, 17.0385},
		{"Poznań", 52.4064, 16.9252},
	}},
	{Code: "DE", Name: "Niemcy", Cities: []City{
		{"Berlin", 52.5200, 13.4050},
		{"Monachium", 48.1351, 11.5820},
		{"Hamburg", 53.5511, 9.9937},
	}},
	{Code: "FR", Name: "Francja", Cities: []City{
		{"Paryż", 48.8566, 2.3522},
		{"Marsylia", 43.2965, 5.3698},
	}},
	{Code: "GB", Name: "Wielka Brytania", Cities: []City{
		{"Londyn", 51.5074, -0.1278},
		{"Manchester", 53.4808, -2.2426},
	}},
	{Code: "US", Name: "USA", Cities: []City{
		{"Nowy Jork", 40.7128, -74.0060},
		{"Chicago", 41.8781, -87.6298},
		{"San Francisco", 37.7749, -122.4194},
	}},
	{Code: "JP", Name: "Japonia", Cities: []City{
		{"Tokio", 35.6762, 139.6503},
	}},
	{Code: "ES", Name: "Hiszpania", Cities: []City{
		{"Madryt", 40.4168, -3.7038},
		{"Barcelona", 41.3851, 2.1734},
	}},
}

// findCity zwraca obiekt City dla pary (kod kraju, nazwa miasta).
func findCity(countryCode, cityName string) (Country, City, bool) {
	for _, c := range countries {
		if c.Code != countryCode {
			continue
		}
		for _, city := range c.Cities {
			if city.Name == cityName {
				return c, city, true
			}
		}
	}
	return Country{}, City{}, false
}

// openMeteoResponse - fragment odpowiedzi API Open-Meteo (current weather).
type openMeteoResponse struct {
	Current struct {
		Time              string  `json:"time"`
		Temperature2m     float64 `json:"temperature_2m"`
		RelativeHumidity  int     `json:"relative_humidity_2m"`
		WindSpeed10m      float64 `json:"wind_speed_10m"`
		WeatherCode       int     `json:"weather_code"`
		ApparentTemp      float64 `json:"apparent_temperature"`
		Precipitation     float64 `json:"precipitation"`
	} `json:"current"`
	CurrentUnits struct {
		Temperature2m    string `json:"temperature_2m"`
		RelativeHumidity string `json:"relative_humidity_2m"`
		WindSpeed10m     string `json:"wind_speed_10m"`
		Precipitation    string `json:"precipitation"`
	} `json:"current_units"`
	Timezone string `json:"timezone"`
}

// weatherCodeDescription - mapowanie WMO Weather Codes na opis (PL).
// Pełna lista: https://open-meteo.com/en/docs (sekcja Weather variable documentation).
var weatherCodeDescription = map[int]string{
	0:  "Słonecznie",
	1:  "Głównie pogodnie",
	2:  "Częściowe zachmurzenie",
	3:  "Pochmurno",
	45: "Mgła",
	48: "Mgła osadzająca szron",
	51: "Mżawka (lekka)",
	53: "Mżawka (umiarkowana)",
	55: "Mżawka (gęsta)",
	61: "Deszcz (słaby)",
	63: "Deszcz (umiarkowany)",
	65: "Deszcz (silny)",
	71: "Śnieg (słaby)",
	73: "Śnieg (umiarkowany)",
	75: "Śnieg (silny)",
	77: "Ziarna śnieżne",
	80: "Przelotny deszcz (słaby)",
	81: "Przelotny deszcz (umiarkowany)",
	82: "Przelotny deszcz (gwałtowny)",
	85: "Przelotny śnieg (słaby)",
	86: "Przelotny śnieg (silny)",
	95: "Burza",
	96: "Burza z lekkim gradem",
	99: "Burza z silnym gradem",
}

// fetchWeather wywołuje Open-Meteo i zwraca surowe dane do dalszego renderowania.
func fetchWeather(ctx context.Context, lat, lon float64) (*openMeteoResponse, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,relative_humidity_2m,apparent_temperature,"+
			"precipitation,weather_code,wind_speed_10m&timezone=auto",
		lat, lon,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo: status %d", resp.StatusCode)
	}
	var out openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HTML szablony - proste, zaszyte w binarce, bez plików zewnętrznych.

const indexTmpl = `<!doctype html>
<html lang="pl"><head>
<meta charset="utf-8">
<title>Zadanie 1 - Pogoda</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
 body{font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;max-width:560px;margin:2rem auto;padding:0 1rem;background:#0f172a;color:#e2e8f0}
 h1{font-size:1.4rem;margin-bottom:.4rem}
 .sub{color:#94a3b8;margin-bottom:1.5rem}
 form{background:#1e293b;padding:1.25rem;border-radius:.6rem;display:grid;gap:.8rem}
 label{font-size:.85rem;color:#94a3b8}
 select,button{padding:.55rem;border-radius:.4rem;border:1px solid #334155;background:#0f172a;color:#e2e8f0;font-size:.95rem}
 button{cursor:pointer;background:#2563eb;border-color:#2563eb;font-weight:600}
 button:hover{background:#1d4ed8}
 footer{margin-top:1.5rem;font-size:.8rem;color:#64748b}
</style></head><body>
<h1>🌤️ Aplikacja pogodowa</h1>
<p class="sub">Wybierz kraj i miasto, aby sprawdzić aktualną pogodę. Dane: <a href="https://open-meteo.com" style="color:#60a5fa">Open-Meteo</a>.</p>
<form action="/weather" method="get">
 <div>
  <label for="country">Kraj</label><br>
  <select id="country" name="country" required onchange="updateCities()">
   <option value="">- wybierz -</option>
   {{range .Countries}}<option value="{{.Code}}">{{.Name}}</option>{{end}}
  </select>
 </div>
 <div>
  <label for="city">Miasto</label><br>
  <select id="city" name="city" required>
   <option value="">- najpierw kraj -</option>
  </select>
 </div>
 <button type="submit">Sprawdź pogodę</button>
</form>
<footer>Autor: {{.Author}}</footer>
<script>
 const data = {{.CitiesJSON}};
 function updateCities(){
  const cs = document.getElementById('city');
  const code = document.getElementById('country').value;
  cs.innerHTML = '<option value="">- wybierz -</option>';
  (data[code]||[]).forEach(n => {
   const o = document.createElement('option'); o.value=n; o.textContent=n; cs.appendChild(o);
  });
 }
</script>
</body></html>`

const weatherTmpl = `<!doctype html>
<html lang="pl"><head>
<meta charset="utf-8">
<title>Pogoda: {{.City}}, {{.Country}}</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
 body{font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;max-width:560px;margin:2rem auto;padding:0 1rem;background:#0f172a;color:#e2e8f0}
 .card{background:#1e293b;border-radius:.6rem;padding:1.25rem;display:grid;gap:.5rem}
 h1{margin:0 0 .2rem 0;font-size:1.4rem}
 .loc{color:#94a3b8;margin-bottom:.6rem}
 .temp{font-size:2.4rem;font-weight:700;color:#fbbf24}
 .row{display:flex;justify-content:space-between;border-bottom:1px solid #334155;padding:.4rem 0}
 .row:last-child{border:none}
 .key{color:#94a3b8}
 a{color:#60a5fa}
 footer{margin-top:1.5rem;font-size:.8rem;color:#64748b}
</style></head><body>
<div class="card">
 <h1>{{.Description}}</h1>
 <div class="loc">{{.City}}, {{.Country}} • {{.Timezone}}</div>
 <div class="temp">{{printf "%.1f" .TempC}} °C</div>
 <div class="row"><span class="key">Odczuwalna</span><span>{{printf "%.1f" .ApparentC}} °C</span></div>
 <div class="row"><span class="key">Wilgotność</span><span>{{.Humidity}} %</span></div>
 <div class="row"><span class="key">Wiatr</span><span>{{printf "%.1f" .WindKmh}} km/h</span></div>
 <div class="row"><span class="key">Opady</span><span>{{printf "%.1f" .PrecipMm}} mm</span></div>
 <div class="row"><span class="key">Pomiar</span><span>{{.Time}}</span></div>
</div>
<p style="margin-top:1rem"><a href="/">← wróć do wyboru</a></p>
<footer>Autor: {{.Author}} • Źródło: Open-Meteo</footer>
</body></html>`

func main() {
	healthcheck := flag.Bool("healthcheck", false, "wykonuje wewnętrzny healthcheck (exit 0/1) i kończy działanie")
	flag.Parse()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if *healthcheck {
		// Tryb healthcheck używany przez Docker HEALTHCHECK w obrazie scratch
		// (brak curl/wget - wykorzystujemy samą binarkę).
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + port + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Wymagane logi startowe (punkt 1a zadania).
	startTime := time.Now().Format(time.RFC3339)
	log.SetFlags(0)
	log.Printf("== Zadanie 1 - start aplikacji ==")
	log.Printf("Data uruchomienia : %s", startTime)
	log.Printf("Autor             : %s", Author)
	log.Printf("Port TCP (nasłuch): %s", port)

	// Przygotowanie danych dla szablonu strony głównej (mapa kod->miasta jako JSON do JS).
	citiesByCountry := map[string][]string{}
	for _, c := range countries {
		names := make([]string, len(c.Cities))
		for i, ci := range c.Cities {
			names[i] = ci.Name
		}
		citiesByCountry[c.Code] = names
	}
	citiesJSON, _ := json.Marshal(citiesByCountry)

	tmplIndex := template.Must(template.New("index").Parse(indexTmpl))
	tmplWeather := template.Must(template.New("weather").Parse(weatherTmpl))

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplIndex.Execute(w, map[string]any{
			"Countries":  countries,
			"Author":     Author,
			"CitiesJSON": template.JS(citiesJSON),
		})
	})

	mux.HandleFunc("/weather", func(w http.ResponseWriter, r *http.Request) {
		country := r.URL.Query().Get("country")
		city := r.URL.Query().Get("city")
		c, loc, ok := findCity(country, city)
		if !ok {
			http.Error(w, "Nieznana lokalizacja: "+country+"/"+city, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		data, err := fetchWeather(ctx, loc.Latitude, loc.Longitude)
		if err != nil {
			http.Error(w, "Błąd pobierania pogody: "+err.Error(), http.StatusBadGateway)
			return
		}
		desc := weatherCodeDescription[data.Current.WeatherCode]
		if desc == "" {
			desc = fmt.Sprintf("Kod pogody %d", data.Current.WeatherCode)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplWeather.Execute(w, map[string]any{
			"City":        loc.Name,
			"Country":     c.Name,
			"Timezone":    data.Timezone,
			"Description": desc,
			"TempC":       data.Current.Temperature2m,
			"ApparentC":   data.Current.ApparentTemp,
			"Humidity":    data.Current.RelativeHumidity,
			"WindKmh":     data.Current.WindSpeed10m,
			"PrecipMm":    data.Current.Precipitation,
			"Time":        data.Current.Time,
			"Author":      Author,
		})
	})

	addr := ":" + port
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Wcześniejsze zbindowanie portu (przed ListenAndServe), aby ewentualny błąd
	// pokazał się od razu w logach startowych.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Nie można nasłuchiwać na %s: %v", addr, err)
	}
	log.Printf("Serwer gotowy: http://0.0.0.0%s/", addr)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Błąd serwera: %v", err)
	}
}
