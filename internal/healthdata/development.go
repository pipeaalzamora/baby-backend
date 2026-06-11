package healthdata

import "fmt"

const (
	DevelopmentAdviceVersion = "desarrollo-infantil-es-2026-06"
	DevelopmentCDCURL        = "https://www.cdc.gov/act-early/es/milestones/index.html"
	DevelopmentChileURL      = "https://www.crececontigo.gob.cl/"
	DevelopmentDentalURL     = "https://www.mouthhealthy.org/all-topics-a-z/eruption-charts"
	DevelopmentPottyURL      = "https://www.healthychildren.org/English/ages-stages/toddler/toilet-training/Pages/The-Right-Age-to-Toilet-Train.aspx"
)

type AdviceSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type AdviceSection struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Items    []string `json:"items"`
	Tips     []string `json:"tips"`
}

type AgeAdvice struct {
	AgeMonths  int             `json:"ageMonths"`
	AgeLabel   string          `json:"ageLabel"`
	Version    string          `json:"version"`
	Summary    string          `json:"summary"`
	Sections   []AdviceSection `json:"sections"`
	RedFlags   []string        `json:"redFlags"`
	NextMonths []int           `json:"nextMonths"`
	Sources    []AdviceSource  `json:"sources"`
}

type adviceTemplate struct {
	AgeMonths int
	Summary   string
	Sections  []AdviceSection
	RedFlags  []string
}

func GetAgeAdvice(ageMonths int) AgeAdvice {
	if ageMonths < 0 {
		ageMonths = 0
	}
	if ageMonths > 60 {
		ageMonths = 60
	}

	template := closestAdviceTemplate(ageMonths)
	return AgeAdvice{
		AgeMonths:  ageMonths,
		AgeLabel:   ageLabel(ageMonths),
		Version:    DevelopmentAdviceVersion,
		Summary:    template.Summary,
		Sections:   template.Sections,
		RedFlags:   template.RedFlags,
		NextMonths: nearbyAdviceMonths(ageMonths),
		Sources: []AdviceSource{
			{Name: "CDC - Indicadores del desarrollo", URL: DevelopmentCDCURL},
			{Name: "Chile Crece Contigo", URL: DevelopmentChileURL},
			{Name: "ADA - Erupción de dientes temporales", URL: DevelopmentDentalURL},
			{Name: "HealthyChildren/AAP - Control de esfínter", URL: DevelopmentPottyURL},
		},
	}
}

func closestAdviceTemplate(ageMonths int) adviceTemplate {
	templates := adviceTemplates()
	best := templates[0]
	for _, item := range templates {
		if absInt(item.AgeMonths-ageMonths) < absInt(best.AgeMonths-ageMonths) {
			best = item
		}
	}
	return best
}

func nearbyAdviceMonths(ageMonths int) []int {
	points := []int{6, 9, 12, 15, 18, 24, 30, 36}
	next := make([]int, 0, 3)
	for _, point := range points {
		if point > ageMonths {
			next = append(next, point)
		}
		if len(next) == 3 {
			break
		}
	}
	return next
}

func adviceTemplates() []adviceTemplate {
	return []adviceTemplate{
		{
			AgeMonths: 6,
			Summary:   "Etapa de mucha exploración: se ríe, reconoce personas familiares, usa la boca para conocer objetos y suele iniciar alimentación complementaria si está listo.",
			Sections: []AdviceSection{
				{
					ID:       "development-6",
					Title:    "Desarrollo esperado",
					Category: "desarrollo",
					Items: []string{
						"Puede reconocer personas conocidas y disfrutar mirarse al espejo.",
						"Se turna haciendo sonidos, balbucea y hace sonidos de placer.",
						"Explora llevándose objetos a la boca y estira el brazo para alcanzar juguetes.",
						"Puede voltearse boca abajo a boca arriba y apoyarse con las manos al sentarse.",
					},
					Tips: []string{
						"Jugar a responder sonidos: si balbucea, copia el sonido y espera su respuesta.",
						"Leer mirando imágenes y nombrar objetos cotidianos.",
						"Jugar en el suelo con juguetes apenas fuera de alcance.",
					},
				},
				{
					ID:       "feeding-6",
					Title:    "Alimentación",
					Category: "alimentación",
					Items: []string{
						"La leche materna o fórmula sigue siendo la base de la alimentación.",
						"Si el equipo de salud indica iniciar sólidos, partir con papillas suaves y alimentos simples.",
						"Observar señales de hambre y saciedad: abrir la boca, mirar la cuchara, cerrar labios o girar la cabeza.",
					},
					Tips: []string{
						"Ofrecer una textura lisa, sin sal, azúcar ni miel.",
						"Introducir alimentos de a poco y registrar tolerancia.",
					},
				},
				{
					ID:       "teeth-6",
					Title:    "Dientes",
					Category: "salud oral",
					Items: []string{
						"Los dientes temporales suelen empezar a salir alrededor de los 6 meses, aunque hay variación normal.",
						"Puede haber más saliva, necesidad de morder objetos seguros o encías sensibles.",
					},
					Tips: []string{
						"Limpiar encías y primeros dientes con gasa o cepillo suave.",
						"Evitar geles o medicamentos sin indicación profesional.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si pierde habilidades que ya tenía.",
				"Consultar si no responde a sonidos, no mira a cuidadores o hay preocupación persistente.",
			},
		},
		{
			AgeMonths: 9,
			Summary:   "Aumenta la interacción: busca objetos, reconoce su nombre, puede sentarse sin apoyo y empieza a practicar comer con los dedos.",
			Sections: []AdviceSection{
				{
					ID:       "development-9",
					Title:    "Desarrollo esperado",
					Category: "desarrollo",
					Items: []string{
						"Puede mostrarse tímido o inseguro con extraños.",
						"Reacciona a su nombre y muestra varias expresiones faciales.",
						"Hace sonidos como 'mamama' o 'bababa'.",
						"Busca objetos que caen y golpea objetos entre sí.",
						"Puede sentarse sin apoyo y pasar objetos de una mano a otra.",
					},
					Tips: []string{
						"Jugar a esconderse y aparecer.",
						"Ofrecer bloques o recipientes para meter y sacar objetos.",
						"Nombrar emociones: contento, triste, sorprendido.",
					},
				},
				{
					ID:       "feeding-9",
					Title:    "Texturas",
					Category: "alimentación",
					Items: []string{
						"Puede avanzar de puré fino a puré grueso o picado muy fino si lo tolera.",
						"Puede practicar comer con dedos con alimentos blandos y seguros.",
					},
					Tips: []string{
						"Evitar alimentos duros, redondos o pegajosos por riesgo de atoramiento.",
						"Sentarse cerca y acompañar toda la comida.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si no se sienta con apoyo o no intenta interactuar con sonidos/miradas.",
				"Consultar si hay atoros frecuentes o rechazo persistente a texturas.",
			},
		},
		{
			AgeMonths: 12,
			Summary:   "Cerca del año aumenta la intención comunicativa y la movilidad. La comida familiar puede adaptarse sin sal ni azúcar añadida.",
			Sections: []AdviceSection{
				{
					ID:       "development-12",
					Title:    "Desarrollo esperado",
					Category: "desarrollo",
					Items: []string{
						"Puede decir gestos como chao o pedir ayuda con movimientos.",
						"Puede entender instrucciones simples con apoyo de gestos.",
						"Puede ponerse de pie con apoyo o desplazarse afirmándose.",
					},
					Tips: []string{
						"Nombrar lo que hace y esperar su respuesta.",
						"Ofrecer espacios seguros para moverse.",
					},
				},
				{
					ID:       "teeth-12",
					Title:    "Dientes y boca",
					Category: "salud oral",
					Items: []string{
						"Entre 6 y 12 meses suele aparecer la primera dentición temporal en muchos bebés.",
						"Al año conviene reforzar higiene oral y consultar controles odontológicos según red de salud.",
					},
					Tips: []string{
						"Usar cepillo suave; evitar dormir con mamadera con líquidos azucarados.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si no responde a su nombre o pierde habilidades.",
				"Consultar si no puede sentarse o sostenerse de ninguna forma.",
			},
		},
		{
			AgeMonths: 18,
			Summary:   "Suele aumentar la autonomía: explora, imita, señala y puede empezar a mostrar señales iniciales para dejar pañales, sin apurar.",
			Sections: []AdviceSection{
				{
					ID:       "development-18",
					Title:    "Autonomía y lenguaje",
					Category: "desarrollo",
					Items: []string{
						"Puede imitar tareas simples y mostrar preferencias.",
						"Puede señalar para pedir o mostrar algo interesante.",
						"Puede caminar con más seguridad y explorar más.",
					},
					Tips: []string{
						"Dar instrucciones simples de un paso.",
						"Ofrecer opciones limitadas: este vaso o este plato.",
					},
				},
				{
					ID:       "potty-18",
					Title:    "Pañales",
					Category: "control de esfínter",
					Items: []string{
						"Algunos niños empiezan a mostrar control de vejiga e intestino entre 18 y 24 meses.",
						"Señales: pasar seco por más tiempo, avisar después de hacer, incomodarse con pañal mojado o interesarse por el baño.",
					},
					Tips: []string{
						"Presentar la pelela sin presión y con lenguaje positivo.",
						"No castigar accidentes ni comparar con otros niños.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si no camina, no señala o no intenta comunicarse.",
				"Consultar si hay estreñimiento doloroso antes de iniciar pelela.",
			},
		},
		{
			AgeMonths: 24,
			Summary:   "A los 2 años muchos niños combinan más palabras, juegan imitando y pueden estar listos para practicar pelela si muestran señales.",
			Sections: []AdviceSection{
				{
					ID:       "development-24",
					Title:    "Lenguaje y juego",
					Category: "desarrollo",
					Items: []string{
						"Puede usar frases cortas y seguir instrucciones simples.",
						"Juega imitando acciones de adultos.",
						"Quiere hacer cosas solo, aunque necesita ayuda.",
					},
					Tips: []string{
						"Leer cuentos cortos y pedirle que señale imágenes.",
						"Nombrar rutinas: primero lavamos manos, después comemos.",
					},
				},
				{
					ID:       "potty-24",
					Title:    "Dejar pañales",
					Category: "control de esfínter",
					Items: []string{
						"El promedio de inicio del entrenamiento suele estar entre 2 y 3 años, pero importan más las señales que la edad.",
						"Necesita entender instrucciones, caminar al baño, ayudar a bajarse ropa y comunicar ganas.",
					},
					Tips: []string{
						"Hacer visitas rutinarias a la pelela: al despertar, antes del baño o antes de dormir.",
						"Elegir palabras familiares y neutras para pipí y deposiciones.",
						"Mantener el proceso positivo; si hay lucha de poder, pausar y reintentar semanas después.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si no usa palabras para comunicarse o pierde lenguaje.",
				"Consultar si hay dolor al orinar, sangre, estreñimiento importante o miedo intenso al baño.",
			},
		},
		{
			AgeMonths: 30,
			Summary:   "Entre 2 años y medio y 3 años suelen consolidarse lenguaje, juego simbólico y habilidades para practicar el baño con más participación.",
			Sections: []AdviceSection{
				{
					ID:       "development-30",
					Title:    "Juego y comunicación",
					Category: "desarrollo",
					Items: []string{
						"Puede jugar a representar situaciones simples.",
						"Puede seguir rutinas con apoyo visual o verbal.",
						"Puede expresar más claramente necesidades básicas.",
					},
					Tips: []string{
						"Usar canciones o pasos visuales para rutinas.",
						"Practicar turnos y pedir ayuda con palabras simples.",
					},
				},
				{
					ID:       "potty-30",
					Title:    "Pelela sin presión",
					Category: "control de esfínter",
					Items: []string{
						"Los accidentes son normales durante el aprendizaje.",
						"El control nocturno suele tardar más que el diurno.",
					},
					Tips: []string{
						"Poner banquito para apoyar pies si usa adaptador de WC.",
						"Elogiar el esfuerzo específico, no usar premios o castigos como centro del proceso.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si el proceso provoca ansiedad intensa o estreñimiento.",
				"Consultar si hay retroceso brusco asociado a dolor o enfermedad.",
			},
		},
		{
			AgeMonths: 36,
			Summary:   "A los 3 años suele haber mayor juego social, lenguaje más claro y muchas familias están en entrenamiento o consolidación del baño.",
			Sections: []AdviceSection{
				{
					ID:       "development-36",
					Title:    "3 años",
					Category: "desarrollo",
					Items: []string{
						"Puede seguir rutinas simples y participar más en autocuidado.",
						"Puede jugar con otros niños con más intención social.",
						"Puede contar lo que necesita con frases más claras.",
					},
					Tips: []string{
						"Reforzar hábitos: lavar manos, guardar juguetes y turnarse.",
						"Usar cuentos para preparar cambios grandes.",
					},
				},
				{
					ID:       "potty-36",
					Title:    "Baño",
					Category: "control de esfínter",
					Items: []string{
						"Muchos niños logran control diurno entre 2 y 4 años.",
						"Las recaídas pueden aparecer con estrés, mudanzas, enfermedad o cambios familiares.",
					},
					Tips: []string{
						"Mantener rutina, ropa fácil de bajar y acceso cómodo.",
						"Evitar vergüenza o retos; resolver accidentes con calma.",
					},
				},
			},
			RedFlags: []string{
				"Consultar si hay dolor, sangre, estreñimiento persistente o infecciones urinarias.",
				"Consultar si no logra comunicarse funcionalmente o hay pérdida de habilidades.",
			},
		},
	}
}

func ageLabel(months int) string {
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

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
