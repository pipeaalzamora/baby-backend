package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const (
	pharmaciesTurnURL  = "https://midas.minsal.cl/farmacia_v2/WS/getLocalesTurnos.php"
	pharmaciesAllURL   = "https://midas.minsal.cl/farmacia_v2/WS/getLocales.php"
	healthPackageURL   = "https://datos.gob.cl/api/3/action/package_show?id=establecimientos-de-salud-vigentes"
	medicineCSVURL     = "https://datos.gob.cl/uploads/recursos/Productos_farmaceuticos_vigentes_venta_directa.csv"
	defaultResultLimit = 30
)

type ExternalHandler struct {
	client *http.Client

	mu              sync.Mutex
	pharmacyCaches  map[string]pharmacyCache
	healthCenters   []HealthCenter
	healthSource    SourceInfo
	healthFetchedAt time.Time
	medicines       []MedicineRecord
	medicineFetched time.Time
}

func NewExternalHandler() *ExternalHandler {
	return &ExternalHandler{
		client:         &http.Client{Timeout: 20 * time.Second},
		pharmacyCaches: map[string]pharmacyCache{},
	}
}

type SourceInfo struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	UpdatedAt  string    `json:"updatedAt,omitempty"`
	FetchedAt  time.Time `json:"fetchedAt"`
	License    string    `json:"license,omitempty"`
	Disclaimer string    `json:"disclaimer,omitempty"`
}

type listResponse struct {
	Source SourceInfo `json:"source"`
	Count  int        `json:"count"`
	Items  any        `json:"items"`
}

type Pharmacy struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	Commune   string `json:"commune"`
	Locality  string `json:"locality"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	OpenTime  string `json:"openTime"`
	CloseTime string `json:"closeTime"`
	Date      string `json:"date"`
	Day       string `json:"day"`
}

type pharmacyRaw struct {
	ID        string `json:"local_id"`
	Name      string `json:"local_nombre"`
	Region    string `json:"fk_region"`
	Commune   string `json:"comuna_nombre"`
	Locality  string `json:"localidad_nombre"`
	Address   string `json:"local_direccion"`
	Phone     string `json:"local_telefono"`
	Latitude  string `json:"local_lat"`
	Longitude string `json:"local_lng"`
	OpenTime  string `json:"funcionamiento_hora_apertura"`
	CloseTime string `json:"funcionamiento_hora_cierre"`
	Date      string `json:"fecha"`
	Day       string `json:"funcionamiento_dia"`
}

type pharmacyCache struct {
	items     []Pharmacy
	source    SourceInfo
	fetchedAt time.Time
}

type HealthCenter struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Region       string `json:"region"`
	Commune      string `json:"commune"`
	Address      string `json:"address"`
	Phone        string `json:"phone"`
	HasEmergency string `json:"hasEmergency"`
	UrgencyType  string `json:"urgencyType"`
	Latitude     string `json:"latitude"`
	Longitude    string `json:"longitude"`
	System       string `json:"system"`
	Status       string `json:"status"`
	Level        string `json:"level"`
}

type MedicineRecord struct {
	Registration  string `json:"registration"`
	Name          string `json:"name"`
	Holder        string `json:"holder"`
	SaleCondition string `json:"saleCondition"`
}

// Pharmacies returns MINSAL Farmanet pharmacy data.
func (h *ExternalHandler) Pharmacies(c *gin.Context) {
	mode := strings.ToLower(c.DefaultQuery("mode", "turnos"))
	if mode != "all" {
		mode = "turnos"
	}
	items, source, err := h.fetchPharmacies(c.Request.Context(), mode)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar farmacias MINSAL"})
		return
	}

	commune := normalizeSearch(c.Query("comuna"))
	region := normalizeSearch(c.Query("region"))
	query := normalizeSearch(c.Query("search"))
	filtered := make([]Pharmacy, 0, len(items))
	for _, item := range items {
		if commune != "" && !strings.Contains(normalizeSearch(item.Commune), commune) {
			continue
		}
		if region != "" && normalizeSearch(item.Region) != region {
			continue
		}
		text := normalizeSearch(item.Name + " " + item.Address + " " + item.Commune + " " + item.Locality)
		if query != "" && !strings.Contains(text, query) {
			continue
		}
		filtered = append(filtered, item)
	}
	filtered = limitItems(filtered, parseLimit(c.Query("limit")))
	c.JSON(http.StatusOK, listResponse{Source: source, Count: len(filtered), Items: filtered})
}

// HealthCenters returns current DEIS health establishments from Datos.gob.
func (h *ExternalHandler) HealthCenters(c *gin.Context) {
	items, source, err := h.fetchHealthCenters(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar establecimientos DEIS"})
		return
	}

	commune := normalizeSearch(c.Query("comuna"))
	region := normalizeSearch(c.Query("region"))
	query := normalizeSearch(c.Query("search"))
	filtered := make([]HealthCenter, 0, len(items))
	for _, item := range items {
		if commune != "" && !strings.Contains(normalizeSearch(item.Commune), commune) {
			continue
		}
		if region != "" && !strings.Contains(normalizeSearch(item.Region), region) {
			continue
		}
		text := normalizeSearch(item.Name + " " + item.Type + " " + item.Commune + " " + item.Region + " " + item.Address)
		if query != "" && !strings.Contains(text, query) {
			continue
		}
		filtered = append(filtered, item)
	}
	filtered = limitItems(filtered, parseLimit(c.Query("limit")))
	c.JSON(http.StatusOK, listResponse{Source: source, Count: len(filtered), Items: filtered})
}

// MedicineRegistry searches the ISP public CSV for direct-sale medicines.
func (h *ExternalHandler) MedicineRegistry(c *gin.Context) {
	items, source, err := h.fetchMedicines(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar registro ISP"})
		return
	}

	query := normalizeSearch(c.Query("search"))
	if len(query) < 2 {
		c.JSON(http.StatusOK, listResponse{Source: source, Count: 0, Items: []MedicineRecord{}})
		return
	}

	filtered := make([]MedicineRecord, 0, 30)
	for _, item := range items {
		text := normalizeSearch(item.Registration + " " + item.Name + " " + item.Holder + " " + item.SaleCondition)
		if strings.Contains(text, query) {
			filtered = append(filtered, item)
		}
	}
	filtered = limitItems(filtered, parseLimit(c.Query("limit")))
	c.JSON(http.StatusOK, listResponse{Source: source, Count: len(filtered), Items: filtered})
}

func (h *ExternalHandler) fetchPharmacies(ctx context.Context, mode string) ([]Pharmacy, SourceInfo, error) {
	h.mu.Lock()
	cache, ok := h.pharmacyCaches[mode]
	if ok && time.Since(cache.fetchedAt) < time.Hour {
		h.mu.Unlock()
		return cache.items, cache.source, nil
	}
	h.mu.Unlock()

	url := pharmaciesTurnURL
	name := "MINSAL Farmanet - farmacias de turno"
	if mode == "all" {
		url = pharmaciesAllURL
		name = "MINSAL Farmanet - locales"
	}

	var raws []pharmacyRaw
	if err := h.getJSON(ctx, url, &raws); err != nil {
		return nil, SourceInfo{}, err
	}

	items := make([]Pharmacy, 0, len(raws))
	for _, raw := range raws {
		items = append(items, Pharmacy{
			ID:        strings.TrimSpace(raw.ID),
			Name:      strings.TrimSpace(raw.Name),
			Region:    strings.TrimSpace(raw.Region),
			Commune:   strings.TrimSpace(raw.Commune),
			Locality:  strings.TrimSpace(raw.Locality),
			Address:   strings.TrimSpace(raw.Address),
			Phone:     strings.TrimSpace(raw.Phone),
			Latitude:  strings.TrimSpace(raw.Latitude),
			Longitude: strings.TrimSpace(raw.Longitude),
			OpenTime:  strings.TrimSpace(raw.OpenTime),
			CloseTime: strings.TrimSpace(raw.CloseTime),
			Date:      strings.TrimSpace(raw.Date),
			Day:       strings.TrimSpace(raw.Day),
		})
	}

	source := SourceInfo{
		Name:       name,
		URL:        url,
		FetchedAt:  time.Now(),
		Disclaimer: "Información pública MINSAL; confirmar horarios en caso de urgencia.",
	}
	h.mu.Lock()
	h.pharmacyCaches[mode] = pharmacyCache{items: items, source: source, fetchedAt: time.Now()}
	h.mu.Unlock()
	return items, source, nil
}

func (h *ExternalHandler) fetchHealthCenters(ctx context.Context) ([]HealthCenter, SourceInfo, error) {
	h.mu.Lock()
	if len(h.healthCenters) > 0 && time.Since(h.healthFetchedAt) < 24*time.Hour {
		items := h.healthCenters
		source := h.healthSource
		h.mu.Unlock()
		return items, source, nil
	}
	h.mu.Unlock()

	var pkg struct {
		Success bool `json:"success"`
		Result  struct {
			LicenseTitle string `json:"license_title"`
			MetadataMod  string `json:"metadata_modified"`
			Resources    []struct {
				Format       string `json:"format"`
				URL          string `json:"url"`
				Name         string `json:"name"`
				LastModified string `json:"last_modified"`
			} `json:"resources"`
		} `json:"result"`
	}
	if err := h.getJSON(ctx, healthPackageURL, &pkg); err != nil {
		return nil, SourceInfo{}, err
	}

	csvURL := ""
	updatedAt := pkg.Result.MetadataMod
	for _, res := range pkg.Result.Resources {
		if strings.EqualFold(res.Format, "csv") {
			csvURL = res.URL
			if res.LastModified != "" {
				updatedAt = res.LastModified
			}
			break
		}
	}
	if csvURL == "" {
		return nil, SourceInfo{}, io.ErrUnexpectedEOF
	}

	resp, err := h.get(ctx, csvURL)
	if err != nil {
		return nil, SourceInfo{}, err
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, SourceInfo{}, err
	}
	if len(records) == 0 {
		return nil, SourceInfo{}, io.ErrUnexpectedEOF
	}
	header := headerIndex(records[0])
	items := make([]HealthCenter, 0, len(records)-1)
	for _, row := range records[1:] {
		name := field(row, header, "EstablecimientoGlosa")
		status := field(row, header, "EstadoFuncionamiento")
		if name == "" || !strings.Contains(normalizeSearch(status), "vigente") {
			continue
		}
		address := strings.TrimSpace(strings.Join([]string{
			field(row, header, "TipoViaGlosa"),
			field(row, header, "NombreVia"),
			field(row, header, "Numero"),
		}, " "))
		items = append(items, HealthCenter{
			Code:         field(row, header, "EstablecimientoCodigo"),
			Name:         name,
			Type:         field(row, header, "TipoEstablecimientoGlosa"),
			Region:       field(row, header, "RegionGlosa"),
			Commune:      field(row, header, "ComunaGlosa"),
			Address:      strings.Join(strings.Fields(address), " "),
			Phone:        field(row, header, "TelefonoMovil_TelefonoFijo"),
			HasEmergency: field(row, header, "TieneServicioUrgencia"),
			UrgencyType:  field(row, header, "TipoUrgencia"),
			Latitude:     field(row, header, "Latitud"),
			Longitude:    field(row, header, "Longitud"),
			System:       field(row, header, "TipoSistemaSaludGlosa"),
			Status:       status,
			Level:        field(row, header, "NivelAtencionEstabglosa"),
		})
	}

	source := SourceInfo{
		Name:       "DEIS/MINSAL - Establecimientos de Salud vigentes",
		URL:        csvURL,
		UpdatedAt:  updatedAt,
		FetchedAt:  time.Now(),
		License:    pkg.Result.LicenseTitle,
		Disclaimer: "Registro administrativo de establecimientos; verificar red local para disponibilidad de prestaciones.",
	}
	h.mu.Lock()
	h.healthCenters = items
	h.healthSource = source
	h.healthFetchedAt = time.Now()
	h.mu.Unlock()
	return items, source, nil
}

func (h *ExternalHandler) fetchMedicines(ctx context.Context) ([]MedicineRecord, SourceInfo, error) {
	h.mu.Lock()
	if len(h.medicines) > 0 && time.Since(h.medicineFetched) < 24*time.Hour {
		items := h.medicines
		source := SourceInfo{
			Name:       "ISP Chile - productos farmacéuticos vigentes de venta directa",
			URL:        medicineCSVURL,
			FetchedAt:  h.medicineFetched,
			Disclaimer: "No entrega dosis ni indicación clínica; confirmar siempre con pediatra o químico farmacéutico.",
		}
		h.mu.Unlock()
		return items, source, nil
	}
	h.mu.Unlock()

	resp, err := h.get(ctx, medicineCSVURL)
	if err != nil {
		return nil, SourceInfo{}, err
	}
	defer resp.Body.Close()

	decoded := transform.NewReader(resp.Body, charmap.ISO8859_1.NewDecoder())
	reader := csv.NewReader(decoded)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, SourceInfo{}, err
	}
	items := make([]MedicineRecord, 0, len(records))
	for _, row := range records[1:] {
		if len(row) < 4 {
			continue
		}
		items = append(items, MedicineRecord{
			Registration:  strings.TrimSpace(row[0]),
			Name:          strings.TrimSpace(row[1]),
			Holder:        strings.TrimSpace(row[2]),
			SaleCondition: strings.TrimSpace(row[3]),
		})
	}

	source := SourceInfo{
		Name:       "ISP Chile - productos farmacéuticos vigentes de venta directa",
		URL:        medicineCSVURL,
		FetchedAt:  time.Now(),
		Disclaimer: "No entrega dosis ni indicación clínica; confirmar siempre con pediatra o químico farmacéutico.",
	}
	h.mu.Lock()
	h.medicines = items
	h.medicineFetched = time.Now()
	h.mu.Unlock()
	return items, source, nil
}

func (h *ExternalHandler) getJSON(ctx context.Context, url string, out any) error {
	resp, err := h.get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (h *ExternalHandler) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "BabyApp/1.0 (+https://github.com)")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, io.ErrUnexpectedEOF
	}
	return resp, nil
}

func headerIndex(header []string) map[string]int {
	out := map[string]int{}
	for i, value := range header {
		out[strings.TrimSpace(value)] = i
	}
	return out
}

func field(row []string, header map[string]int, name string) string {
	i, ok := header[name]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func normalizeSearch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u",
		"ñ", "n", "Ñ", "n",
	)
	return replacer.Replace(value)
}

func parseLimit(raw string) int {
	if raw == "" {
		return defaultResultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultResultLimit
	}
	if n > 100 {
		return 100
	}
	return n
}

func limitItems[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
