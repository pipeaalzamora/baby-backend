package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const theMealDBBaseURL = "https://www.themealdb.com/api/json/v1"

// RecipeHandler manages baby food recipes and food introductions.
type RecipeHandler struct {
	db              *repository.DB
	client          *http.Client
	theMealDBAPIKey string
}

// NewRecipeHandler creates a new RecipeHandler.
func NewRecipeHandler(db *repository.DB, theMealDBAPIKey string) *RecipeHandler {
	if strings.TrimSpace(theMealDBAPIKey) == "" {
		theMealDBAPIKey = "1"
	}
	return &RecipeHandler{
		db:              db,
		client:          &http.Client{Timeout: 15 * time.Second},
		theMealDBAPIKey: strings.TrimSpace(theMealDBAPIKey),
	}
}

type ExternalRecipe struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Provider     string                    `json:"provider,omitempty"`
	Category     string                    `json:"category,omitempty"`
	Area         string                    `json:"area,omitempty"`
	ImageURL     string                    `json:"imageUrl,omitempty"`
	VideoURL     string                    `json:"videoUrl,omitempty"`
	SourceURL    string                    `json:"sourceUrl,omitempty"`
	MinAgeMonths int                       `json:"minAgeMonths"`
	AgeLabel     string                    `json:"ageLabel"`
	Stage        string                    `json:"stage"`
	Texture      string                    `json:"texture"`
	PrepTimeMin  int                       `json:"prepTimeMin"`
	Ingredients  []models.RecipeIngredient `json:"ingredients"`
	Steps        []string                  `json:"steps"`
	Warnings     []string                  `json:"warnings"`
	Tags         []string                  `json:"tags"`
}

type externalRecipeResponse struct {
	Source          SourceInfo       `json:"source"`
	Query           string           `json:"query"`
	TranslatedQuery string           `json:"translatedQuery,omitempty"`
	AgeMonths       int              `json:"ageMonths"`
	AgeLabel        string           `json:"ageLabel"`
	Mode            string           `json:"mode"`
	Count           int              `json:"count"`
	Items           []ExternalRecipe `json:"items"`
}

type mealDBSearchResponse struct {
	Meals []map[string]any `json:"meals"`
}

// List godoc — GET /api/recipes?stage=6m
func (h *RecipeHandler) List(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	filter := bson.M{"childId": childID}
	if stage := c.Query("stage"); stage != "" {
		filter["stage"] = stage
	}

	col := h.db.Collection(repository.ColRecipes)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "stage", Value: 1}, {Key: "name", Value: 1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Recipe
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Recipe{}
	}
	for i := range items {
		normalizeRecipeSlices(&items[i])
	}
	c.JSON(http.StatusOK, items)
}

// SearchExternal godoc — GET /api/recipes/search-external?q=chicken&ageMonths=12
func (h *RecipeHandler) SearchExternal(c *gin.Context) {
	query := strings.TrimSpace(c.DefaultQuery("q", "papilla"))
	if query == "" {
		query = "papilla"
	}
	if len([]rune(query)) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "busca al menos 2 caracteres"})
		return
	}

	ageMonths := parseRecipeAgeMonths(c.DefaultQuery("ageMonths", "12"))
	limit := parseRecipeLimit(c.DefaultQuery("limit", "8"))
	translated := translateRecipeQuery(query)

	source := SourceInfo{
		Name:       "Recetario local para alimentación complementaria",
		URL:        "https://www.crececontigo.gob.cl/tema/la-lactancia-el-mejor-alimento/",
		FetchedAt:  time.Now(),
		Disclaimer: "Recetas en español para 6 a 10 meses. Sin sal ni azúcar añadida; adaptar textura y alérgenos según indicación profesional.",
	}

	if ageMonths < 6 {
		c.JSON(http.StatusOK, externalRecipeResponse{
			Source:          source,
			Query:           query,
			TranslatedQuery: translated,
			AgeMonths:       ageMonths,
			AgeLabel:        recipeAgeLabel(ageMonths),
			Mode:            "papillas",
			Count:           0,
			Items:           []ExternalRecipe{},
		})
		return
	}

	localItems := filterLocalBabyRecipes(query, ageMonths, limit)
	if ageMonths <= 10 || len(localItems) >= limit {
		c.JSON(http.StatusOK, externalRecipeResponse{
			Source:          source,
			Query:           query,
			TranslatedQuery: translated,
			AgeMonths:       ageMonths,
			AgeLabel:        recipeAgeLabel(ageMonths),
			Mode:            "papillas",
			Count:           len(localItems),
			Items:           localItems,
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	meals, err := h.fetchMealDBSearch(ctx, translated)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar TheMealDB"})
		return
	}
	if len(meals) == 0 {
		meals, err = h.fetchMealDBByIngredient(ctx, translated, limit)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar TheMealDB"})
			return
		}
	}

	items := make([]ExternalRecipe, 0, limit)
	items = append(items, localItems...)
	for _, meal := range meals {
		item := normalizeMealDBRecipe(meal)
		if item.ID == "" || item.Name == "" || len(item.Ingredients) == 0 || len(item.Steps) == 0 {
			continue
		}
		item.MinAgeMonths, item.Warnings = estimateRecipeAge(item)
		item.AgeLabel = recipeAgeLabel(item.MinAgeMonths)
		item.Stage, item.Texture = recipeStageAndTexture(maxInt(ageMonths, item.MinAgeMonths))
		item.PrepTimeMin = estimateRecipePrepTime(item)
		if item.MinAgeMonths > ageMonths {
			continue
		}
		item = translateExternalRecipe(item)
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}

	source = SourceInfo{
		Name:       "Recetario local + TheMealDB",
		URL:        "https://www.themealdb.com/api.php",
		FetchedAt:  time.Now(),
		Disclaimer: "Las recetas externas son generales y se traducen/adaptan automáticamente; revisar siempre textura, sal, azúcar y alérgenos.",
	}

	c.JSON(http.StatusOK, externalRecipeResponse{
		Source:          source,
		Query:           query,
		TranslatedQuery: translated,
		AgeMonths:       ageMonths,
		AgeLabel:        recipeAgeLabel(ageMonths),
		Mode:            "mixto",
		Count:           len(items),
		Items:           items,
	})
}

// Create godoc — POST /api/recipes
func (h *RecipeHandler) Create(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perfil de bebé requerido"})
		return
	}

	var body models.Recipe
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()
	normalizeRecipeSlices(&body)

	col := h.db.Collection(repository.ColRecipes)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res, err := col.InsertOne(ctx, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	body.ID = res.InsertedID.(bson.ObjectID)
	c.JSON(http.StatusCreated, body)
}

func normalizeRecipeSlices(recipe *models.Recipe) {
	if recipe.Ingredients == nil {
		recipe.Ingredients = []models.RecipeIngredient{}
	}
	if recipe.Steps == nil {
		recipe.Steps = []string{}
	}
	if recipe.NutritionHighlights == nil {
		recipe.NutritionHighlights = []string{}
	}
	if recipe.Allergens == nil {
		recipe.Allergens = []string{}
	}
}

func filterLocalBabyRecipes(query string, ageMonths, limit int) []ExternalRecipe {
	normalizedQuery := normalizeSearch(query)
	items := make([]ExternalRecipe, 0, limit)
	for _, recipe := range localBabyPureeRecipes() {
		if recipe.MinAgeMonths > ageMonths {
			continue
		}
		if normalizedQuery != "" && normalizedQuery != "papilla" && normalizedQuery != "pure" {
			text := normalizeSearch(recipe.Name + " " + strings.Join(recipe.Tags, " "))
			for _, ingredient := range recipe.Ingredients {
				text += " " + normalizeSearch(ingredient.Name)
			}
			if !strings.Contains(text, normalizedQuery) {
				continue
			}
		}
		recipe.AgeLabel = recipeAgeLabel(recipe.MinAgeMonths)
		items = append(items, recipe)
		if len(items) >= limit {
			break
		}
	}
	return items
}

func localBabyPureeRecipes() []ExternalRecipe {
	commonWarnings := []string{
		"No agregar sal, azúcar, miel, caldos comerciales ni condimentos picantes.",
		"Ofrecer sentado, con supervisión y textura acorde a la edad.",
	}
	return []ExternalRecipe{
		{
			ID:           "local-papilla-zanahoria-papa-6m",
			Provider:     "local",
			Name:         "Papilla suave de zanahoria y papa",
			Category:     "Papilla",
			MinAgeMonths: 6,
			Stage:        "6m",
			Texture:      "Puré fino",
			PrepTimeMin:  25,
			Ingredients: []models.RecipeIngredient{
				{Name: "Zanahoria pelada", Amount: "1/2 taza"},
				{Name: "Papa pelada", Amount: "1/2 taza"},
				{Name: "Agua de cocción o leche habitual", Amount: "2 a 4 cucharadas"},
				{Name: "Aceite vegetal crudo", Amount: "1 cucharadita opcional"},
			},
			Steps: []string{
				"Lavar, pelar y cortar la zanahoria y la papa en trozos pequeños.",
				"Cocer en agua hasta que estén muy blandas.",
				"Moler hasta lograr un puré fino, agregando agua de cocción o leche habitual de a poco.",
				"Servir tibio y probar temperatura antes de ofrecer.",
			},
			Warnings: append([]string{}, commonWarnings...),
			Tags:     []string{"zanahoria", "papa", "verduras", "6 meses", "papilla"},
		},
		{
			ID:           "local-papilla-manzana-pera-6m",
			Provider:     "local",
			Name:         "Compota de manzana y pera",
			Category:     "Papilla",
			MinAgeMonths: 6,
			Stage:        "6m",
			Texture:      "Puré fino",
			PrepTimeMin:  18,
			Ingredients: []models.RecipeIngredient{
				{Name: "Manzana pelada", Amount: "1/2 unidad"},
				{Name: "Pera pelada", Amount: "1/2 unidad"},
				{Name: "Agua", Amount: "2 cucharadas si hace falta"},
			},
			Steps: []string{
				"Cortar la fruta en trozos pequeños y retirar semillas.",
				"Cocer al vapor o en poca agua hasta que esté blanda.",
				"Moler hasta lograr textura lisa, sin agregar azúcar ni endulzantes.",
				"Servir tibia o fría, en porción pequeña al inicio.",
			},
			Warnings: append([]string{}, commonWarnings...),
			Tags:     []string{"manzana", "pera", "fruta", "6 meses", "compota"},
		},
		{
			ID:           "local-papilla-zapallo-pollo-7m",
			Provider:     "local",
			Name:         "Papilla de zapallo, arroz y pollo",
			Category:     "Papilla con proteína",
			MinAgeMonths: 7,
			Stage:        "8m",
			Texture:      "Puré fino a puré grueso",
			PrepTimeMin:  30,
			Ingredients: []models.RecipeIngredient{
				{Name: "Zapallo camote", Amount: "1/2 taza"},
				{Name: "Arroz cocido", Amount: "2 cucharadas"},
				{Name: "Pollo cocido sin piel", Amount: "2 cucharadas"},
				{Name: "Agua de cocción", Amount: "2 a 4 cucharadas"},
			},
			Steps: []string{
				"Cocer el zapallo y el pollo hasta que estén muy blandos y bien cocidos.",
				"Mezclar con arroz cocido.",
				"Moler todo junto; dejar más liso o más grueso según tolerancia.",
				"Revisar que no queden fibras duras ni trozos grandes.",
			},
			Warnings: append([]string{}, commonWarnings...),
			Tags:     []string{"zapallo", "pollo", "arroz", "proteína", "7 meses", "8 meses"},
		},
		{
			ID:           "local-papilla-lentejas-verduras-8m",
			Provider:     "local",
			Name:         "Papilla de lentejas y verduras",
			Category:     "Papilla con legumbres",
			MinAgeMonths: 8,
			Stage:        "8m",
			Texture:      "Puré grueso",
			PrepTimeMin:  35,
			Ingredients: []models.RecipeIngredient{
				{Name: "Lentejas cocidas", Amount: "1/3 taza"},
				{Name: "Zanahoria cocida", Amount: "2 cucharadas"},
				{Name: "Zapallo cocido", Amount: "2 cucharadas"},
				{Name: "Aceite vegetal crudo", Amount: "1 cucharadita opcional"},
			},
			Steps: []string{
				"Usar lentejas bien cocidas y blandas.",
				"Mezclar con verduras cocidas.",
				"Moler o pasar por cedazo si la cáscara molesta.",
				"Agregar agua de cocción si queda muy espesa.",
			},
			Warnings: append([]string{"Las legumbres pueden producir gases; ofrecer poca cantidad al inicio."}, commonWarnings...),
			Tags:     []string{"lentejas", "legumbres", "verduras", "8 meses", "hierro"},
		},
		{
			ID:           "local-papilla-pescado-papa-8m",
			Provider:     "local",
			Name:         "Papilla de pescado, papa y zanahoria",
			Category:     "Papilla con proteína",
			MinAgeMonths: 8,
			Stage:        "8m",
			Texture:      "Puré grueso",
			PrepTimeMin:  25,
			Ingredients: []models.RecipeIngredient{
				{Name: "Pescado blanco bien cocido", Amount: "2 cucharadas"},
				{Name: "Papa cocida", Amount: "1/2 taza"},
				{Name: "Zanahoria cocida", Amount: "2 cucharadas"},
				{Name: "Agua de cocción", Amount: "2 cucharadas"},
			},
			Steps: []string{
				"Cocer el pescado completamente.",
				"Revisar cuidadosamente y retirar todas las espinas.",
				"Moler con papa y zanahoria hasta lograr textura suave.",
				"Ofrecer poca cantidad la primera vez y observar tolerancia.",
			},
			Warnings: append([]string{"Pescado es posible alérgeno; introducir según pauta familiar/profesional."}, commonWarnings...),
			Tags:     []string{"pescado", "papa", "zanahoria", "8 meses", "proteína"},
		},
		{
			ID:           "local-papilla-avena-platano-8m",
			Provider:     "local",
			Name:         "Avena cocida con plátano molido",
			Category:     "Papilla cereal",
			MinAgeMonths: 8,
			Stage:        "8m",
			Texture:      "Puré grueso",
			PrepTimeMin:  12,
			Ingredients: []models.RecipeIngredient{
				{Name: "Avena tradicional cocida", Amount: "3 cucharadas"},
				{Name: "Plátano maduro", Amount: "1/3 unidad"},
				{Name: "Agua o leche habitual", Amount: "2 a 3 cucharadas"},
			},
			Steps: []string{
				"Cocer la avena en agua hasta que esté blanda.",
				"Moler el plátano con tenedor.",
				"Mezclar y ajustar consistencia con agua o leche habitual.",
				"No agregar azúcar, miel ni endulzantes.",
			},
			Warnings: append([]string{}, commonWarnings...),
			Tags:     []string{"avena", "plátano", "banana", "cereal", "8 meses"},
		},
		{
			ID:           "local-papilla-quinoa-pavo-9m",
			Provider:     "local",
			Name:         "Papilla de quinoa, zapallo y pavo",
			Category:     "Papilla con proteína",
			MinAgeMonths: 9,
			Stage:        "10m",
			Texture:      "Triturado suave",
			PrepTimeMin:  35,
			Ingredients: []models.RecipeIngredient{
				{Name: "Quinoa bien lavada y cocida", Amount: "3 cucharadas"},
				{Name: "Zapallo cocido", Amount: "1/2 taza"},
				{Name: "Pavo cocido", Amount: "2 cucharadas"},
				{Name: "Agua de cocción", Amount: "2 cucharadas"},
			},
			Steps: []string{
				"Lavar la quinoa hasta que el agua salga clara y cocer hasta que esté blanda.",
				"Cocer el pavo completamente y desmenuzar.",
				"Moler o triturar con zapallo.",
				"Dejar textura levemente más gruesa si el bebé ya la tolera.",
			},
			Warnings: append([]string{}, commonWarnings...),
			Tags:     []string{"quinoa", "pavo", "zapallo", "9 meses", "10 meses"},
		},
		{
			ID:           "local-papilla-huevo-verduras-10m",
			Provider:     "local",
			Name:         "Verduras trituradas con huevo bien cocido",
			Category:     "Triturado",
			MinAgeMonths: 10,
			Stage:        "10m",
			Texture:      "Triturado",
			PrepTimeMin:  22,
			Ingredients: []models.RecipeIngredient{
				{Name: "Huevo duro bien cocido", Amount: "1/2 unidad"},
				{Name: "Zapallo italiano cocido", Amount: "1/3 taza"},
				{Name: "Papa o arroz cocido", Amount: "1/3 taza"},
				{Name: "Agua de cocción", Amount: "1 a 2 cucharadas"},
			},
			Steps: []string{
				"Cocer el huevo hasta que yema y clara estén completamente firmes.",
				"Cocer las verduras hasta que estén blandas.",
				"Triturar todo con tenedor, sin dejar trozos duros.",
				"Ofrecer en poca cantidad si es primera exposición al huevo.",
			},
			Warnings: append([]string{"Huevo es posible alérgeno; introducir según pauta familiar/profesional."}, commonWarnings...),
			Tags:     []string{"huevo", "verduras", "zapallo italiano", "10 meses", "proteína"},
		},
	}
}

func (h *RecipeHandler) fetchMealDBSearch(ctx context.Context, query string) ([]map[string]any, error) {
	endpoint := fmt.Sprintf("%s/%s/search.php?s=%s", theMealDBBaseURL, url.PathEscape(h.theMealDBAPIKey), url.QueryEscape(query))
	var response mealDBSearchResponse
	if err := h.getMealDB(ctx, endpoint, &response); err != nil {
		return nil, err
	}
	return response.Meals, nil
}

func (h *RecipeHandler) fetchMealDBByIngredient(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	ingredient := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(query)), " ", "_")
	endpoint := fmt.Sprintf("%s/%s/filter.php?i=%s", theMealDBBaseURL, url.PathEscape(h.theMealDBAPIKey), url.QueryEscape(ingredient))
	var filtered mealDBSearchResponse
	if err := h.getMealDB(ctx, endpoint, &filtered); err != nil {
		return nil, err
	}

	meals := make([]map[string]any, 0, limit)
	for _, item := range filtered.Meals {
		id := mealString(item, "idMeal")
		if id == "" {
			continue
		}
		lookup := fmt.Sprintf("%s/%s/lookup.php?i=%s", theMealDBBaseURL, url.PathEscape(h.theMealDBAPIKey), url.QueryEscape(id))
		var detail mealDBSearchResponse
		if err := h.getMealDB(ctx, lookup, &detail); err != nil {
			return nil, err
		}
		if len(detail.Meals) > 0 {
			meals = append(meals, detail.Meals[0])
		}
		if len(meals) >= limit {
			break
		}
	}
	return meals, nil
}

func (h *RecipeHandler) getMealDB(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("themealdb status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func normalizeMealDBRecipe(meal map[string]any) ExternalRecipe {
	ingredients := make([]models.RecipeIngredient, 0, 12)
	for i := 1; i <= 20; i++ {
		name := mealString(meal, fmt.Sprintf("strIngredient%d", i))
		if name == "" {
			continue
		}
		ingredients = append(ingredients, models.RecipeIngredient{
			Name:   name,
			Amount: mealString(meal, fmt.Sprintf("strMeasure%d", i)),
		})
	}

	steps := splitRecipeSteps(mealString(meal, "strInstructions"))
	tags := splitRecipeTags(mealString(meal, "strTags"))
	category := mealString(meal, "strCategory")
	area := mealString(meal, "strArea")
	if category != "" {
		tags = append(tags, category)
	}
	if area != "" {
		tags = append(tags, area)
	}

	return ExternalRecipe{
		ID:          mealString(meal, "idMeal"),
		Name:        mealString(meal, "strMeal"),
		Provider:    "themealdb",
		Category:    category,
		Area:        area,
		ImageURL:    mealString(meal, "strMealThumb"),
		VideoURL:    mealString(meal, "strYoutube"),
		SourceURL:   mealString(meal, "strSource"),
		Ingredients: ingredients,
		Steps:       steps,
		Tags:        uniqueStrings(tags),
	}
}

func translateExternalRecipe(recipe ExternalRecipe) ExternalRecipe {
	recipe.Name = translateRecipeTitle(recipe.Name)
	recipe.Category = translateRecipeWord(recipe.Category)
	recipe.Area = translateRecipeWord(recipe.Area)
	for i := range recipe.Ingredients {
		recipe.Ingredients[i].Name = translateRecipeIngredient(recipe.Ingredients[i].Name)
		recipe.Ingredients[i].Amount = translateRecipeMeasure(recipe.Ingredients[i].Amount)
	}
	for i := range recipe.Steps {
		recipe.Steps[i] = translateRecipeInstruction(recipe.Steps[i])
	}
	for i := range recipe.Tags {
		recipe.Tags[i] = translateRecipeWord(recipe.Tags[i])
	}
	recipe.Tags = uniqueStrings(append(recipe.Tags, "receta externa", "adaptar para bebé"))
	return recipe
}

func translateRecipeTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	known := map[string]string{
		"chicken soup":                            "Sopa de pollo",
		"fish soup (ukha)":                        "Sopa de pescado",
		"egg drop soup":                           "Sopa con huevo",
		"mushroom soup with buckwheat":            "Sopa de champiñones con trigo sarraceno",
		"clear soup with semolina dumplings":      "Sopa clara con bolitas de sémola",
		"fiskesuppe (creamy norwegian fish soup)": "Sopa cremosa de pescado",
	}
	if translated, ok := known[strings.ToLower(value)]; ok {
		return translated
	}
	return titleCaseSpanish(replaceRecipeTerms(value))
}

func translateRecipeIngredient(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return titleCaseSpanish(replaceRecipeTerms(value))
}

func translateRecipeMeasure(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"tbs", "cda",
		"tbsp", "cda",
		"tablespoon", "cucharada",
		"tablespoons", "cucharadas",
		"tsp", "cdta",
		"teaspoon", "cucharadita",
		"teaspoons", "cucharaditas",
		"cup", "taza",
		"cups", "tazas",
		"chopped", "picado",
		"finely diced", "picado fino",
		"sliced", "en rodajas",
		"pinch", "pizca",
		"large", "grande",
		"medium", "mediano",
		"small", "pequeño",
		"to serve", "para servir",
	)
	return strings.TrimSpace(replacer.Replace(strings.ToLower(value)))
}

func translateRecipeInstruction(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacements := []string{
		"Heat the oil", "Calentar el aceite",
		"Add the", "Agregar",
		"Bring to the boil", "Llevar a hervor",
		"bring to a boil", "llevar a hervor",
		"Reduce the heat", "Bajar el fuego",
		"Simmer", "Cocinar a fuego suave",
		"Cook", "Cocinar",
		"Drain", "Escurrir",
		"Wash", "Lavar",
		"Rinse", "Enjuagar",
		"Chop", "Picar",
		"Serve", "Servir",
		"Season to taste", "Sazonar al gusto",
		"salt", "sal",
		"pepper", "pimienta",
		"water", "agua",
		"until tender", "hasta que esté blando",
		"stir", "revolver",
		"cover", "tapar",
		"minutes", "minutos",
	}
	replacer := strings.NewReplacer(replacements...)
	translated := replacer.Replace(value)
	if translated == value {
		return "Adaptar receta externa: " + replaceRecipeTerms(value)
	}
	return replaceRecipeTerms(translated)
}

func translateRecipeWord(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return titleCaseSpanish(replaceRecipeTerms(value))
}

func replaceRecipeTerms(value string) string {
	replacer := strings.NewReplacer(
		"Chicken", "Pollo", "chicken", "pollo",
		"Turkey", "Pavo", "turkey", "pavo",
		"Fish", "Pescado", "fish", "pescado",
		"Carrot", "Zanahoria", "carrot", "zanahoria",
		"Carrots", "Zanahorias", "carrots", "zanahorias",
		"Potato", "Papa", "potato", "papa",
		"Potatoes", "Papas", "potatoes", "papas",
		"Pumpkin", "Zapallo", "pumpkin", "zapallo",
		"Rice", "Arroz", "rice", "arroz",
		"Oats", "Avena", "oats", "avena",
		"Apple", "Manzana", "apple", "manzana",
		"Pear", "Pera", "pear", "pera",
		"Banana", "Plátano", "banana", "plátano",
		"Egg", "Huevo", "egg", "huevo",
		"Milk", "Leche", "milk", "leche",
		"Yogurt", "Yogur", "yogurt", "yogur",
		"Cheese", "Queso", "cheese", "queso",
		"Onion", "Cebolla", "onion", "cebolla",
		"Garlic", "Ajo", "garlic", "ajo",
		"Peas", "Arvejas", "peas", "arvejas",
		"Beans", "Porotos", "beans", "porotos",
		"Lentil", "Lenteja", "lentil", "lenteja",
		"Lentils", "Lentejas", "lentils", "lentejas",
		"Soup", "Sopa", "soup", "sopa",
		"Vegetarian", "Vegetariana", "vegetarian", "vegetariana",
		"Seafood", "Pescados y mariscos", "seafood", "pescados y mariscos",
		"Beef", "Vacuno", "beef", "vacuno",
		"Pork", "Cerdo", "pork", "cerdo",
		"Side", "Acompañamiento", "side", "acompañamiento",
		"Miscellaneous", "Variada", "miscellaneous", "variada",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func titleCaseSpanish(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Fields(strings.ToLower(value))
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		runes := []rune(part)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func estimateRecipeAge(recipe ExternalRecipe) (int, []string) {
	text := strings.ToLower(recipe.Name + " " + recipe.Category + " " + strings.Join(recipe.Steps, " "))
	for _, ingredient := range recipe.Ingredients {
		text += " " + strings.ToLower(ingredient.Name+" "+ingredient.Amount)
	}

	minAge := 12
	warnings := []string{"Adaptar sin sal añadida, sin azúcar añadida y con textura segura para la edad."}

	if hasAny(text, "porridge", "oat", "oats", "banana", "apple", "carrot", "potato", "rice", "pumpkin", "soup") {
		minAge = 8
	}
	if hasAny(text, "puree", "purée") {
		minAge = 6
	}
	if hasAny(text, "chicken", "turkey", "fish", "lentil", "beans", "pea") && minAge < 8 {
		minAge = 8
	}
	if hasAny(text, "beef", "pork", "lamb", "bacon", "sausage", "ham") {
		minAge = maxInt(minAge, 12)
		warnings = append(warnings, "Contiene carnes o procesados: preferir cortes magros y evitar embutidos para bebé.")
	}
	if hasAny(text, "egg", "milk", "cheese", "yogurt", "fish", "peanut", "nut", "sesame", "soy", "wheat") {
		warnings = append(warnings, "Contiene posibles alérgenos; introducir según pauta familiar/profesional y observar tolerancia.")
	}
	if hasAny(text, "honey") {
		minAge = maxInt(minAge, 12)
		warnings = append(warnings, "Contiene miel o similar: no usar antes de los 12 meses.")
	}
	if hasAny(text, "sugar", "syrup", "chocolate", "cake", "dessert", "cookie", "biscuit", "caramel") {
		minAge = maxInt(minAge, 24)
		warnings = append(warnings, "Receta dulce: limitar azúcar y reservar para mayores según criterio familiar/profesional.")
	}
	if hasAny(text, "salt", "stock cube", "soy sauce", "miso", "bouillon") {
		minAge = maxInt(minAge, 12)
		warnings = append(warnings, "Revisar sodio: preparar versión sin sal añadida para bebé.")
	}
	if hasAny(text, "chili", "chilli", "curry", "pepper", "spicy", "jalapeno", "jalapeño") {
		minAge = maxInt(minAge, 24)
		warnings = append(warnings, "Puede ser picante o muy condimentada; adaptar condimentos.")
	}
	if hasAny(text, "wine", "beer", "vodka", "rum", "whisky", "brandy", "liqueur") {
		minAge = maxInt(minAge, 99)
		warnings = append(warnings, "Contiene alcohol en la receta original; no recomendado para bebé.")
	}
	if hasAny(text, "whole nut", "almond", "walnut", "hazelnut", "cashew") {
		minAge = maxInt(minAge, 36)
		warnings = append(warnings, "Frutos secos enteros o duros implican riesgo de atragantamiento; adaptar solo si corresponde.")
	}

	return minAge, uniqueStrings(warnings)
}

func estimateRecipePrepTime(recipe ExternalRecipe) int {
	minutes := 15 + len(recipe.Steps)*3 + len(recipe.Ingredients)
	if strings.EqualFold(recipe.Category, "Dessert") {
		minutes += 15
	}
	if minutes < 15 {
		return 15
	}
	if minutes > 75 {
		return 75
	}
	return minutes
}

func splitRecipeSteps(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	parts := strings.Split(value, "\n")
	if len(parts) <= 1 {
		parts = strings.Split(value, ". ")
	}
	steps := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, ".")
		if len([]rune(part)) < 4 {
			continue
		}
		steps = append(steps, part)
	}
	return steps
}

func splitRecipeTags(value string) []string {
	raw := strings.Split(value, ",")
	tags := make([]string, 0, len(raw))
	for _, tag := range raw {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func translateRecipeQuery(query string) string {
	normalized := normalizeSearch(query)
	translations := map[string]string{
		"arroz":      "rice",
		"avena":      "oats",
		"banana":     "banana",
		"carne":      "beef",
		"manzana":    "apple",
		"papa":       "potato",
		"patata":     "potato",
		"pavo":       "turkey",
		"pera":       "pear",
		"pescado":    "fish",
		"pollo":      "chicken",
		"pure":       "puree",
		"puré":       "puree",
		"sopa":       "soup",
		"verduras":   "vegetable",
		"zanahoria":  "carrot",
		"zapallo":    "pumpkin",
		"calabaza":   "pumpkin",
		"lentejas":   "lentil",
		"legumbres":  "beans",
		"huevo":      "egg",
		"yogur":      "yogurt",
		"yogurt":     "yogurt",
		"tallarines": "pasta",
		"fideos":     "pasta",
	}
	if translated, ok := translations[normalized]; ok {
		return translated
	}
	return query
}

func parseRecipeAgeMonths(value string) int {
	age, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 12
	}
	if age < 0 {
		return 0
	}
	if age > 72 {
		return 72
	}
	return age
}

func parseRecipeLimit(value string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 8
	}
	if limit < 1 {
		return 1
	}
	if limit > 12 {
		return 12
	}
	return limit
}

func recipeAgeLabel(months int) string {
	if months < 12 {
		return fmt.Sprintf("%d meses", months)
	}
	years := months / 12
	rest := months % 12
	if rest == 0 {
		if years == 1 {
			return "1 año"
		}
		return fmt.Sprintf("%d años", years)
	}
	if years == 1 {
		return fmt.Sprintf("1 año %d meses", rest)
	}
	return fmt.Sprintf("%d años %d meses", years, rest)
}

func recipeStageAndTexture(months int) (string, string) {
	switch {
	case months < 8:
		return "6m", "Puré fino"
	case months < 10:
		return "8m", "Puré grueso"
	case months < 12:
		return "10m", "Triturado"
	default:
		return "12m+", "Trozos pequeños o comida familiar adaptada"
	}
}

func mealString(meal map[string]any, key string) string {
	value, ok := meal[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func hasAny(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, value)
	}
	return unique
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ToggleFavorite godoc — PATCH /api/recipes/:id/favorite
func (h *RecipeHandler) ToggleFavorite(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var body struct {
		IsFavorite bool `json:"isFavorite"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col := h.db.Collection(repository.ColRecipes)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var recipe models.Recipe
	if err := col.FindOneAndUpdate(ctx, bson.M{"_id": id, "childId": childID},
		bson.M{"$set": bson.M{"isFavorite": body.IsFavorite}}, opts).Decode(&recipe); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "receta no encontrada"})
		return
	}
	c.JSON(http.StatusOK, recipe)
}

// ListIntroductions godoc — GET /api/recipes/introductions
func (h *RecipeHandler) ListIntroductions(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	col := h.db.Collection(repository.ColFoodIntroductions)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.FoodIntroduction
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.FoodIntroduction{}
	}
	c.JSON(http.StatusOK, items)
}

// CreateIntroduction godoc — POST /api/recipes/introductions
func (h *RecipeHandler) CreateIntroduction(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perfil de bebé requerido"})
		return
	}

	var body models.FoodIntroduction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()

	col := h.db.Collection(repository.ColFoodIntroductions)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res, err := col.InsertOne(ctx, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	body.ID = res.InsertedID.(bson.ObjectID)
	c.JSON(http.StatusCreated, body)
}
