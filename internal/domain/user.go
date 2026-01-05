package domain

type User struct {
	ID        int64
	ChatID    int64
	Username  string
	CreatedAt string
}

type UserState struct {
	UserID  int64
	Command string
	Step    string
	Data    map[string]interface{}
}

type WaterCalculation struct {
	Weight   float64
	Activity string
	Result   float64
}

type OneRepMaxCalculation struct {
	Weight  float64
	Reps    int
	Formula string
	Result  float64
}

type BodyFatCalculation struct {
	Gender string
	Age    int
	Weight float64
	Height float64
	Neck   float64
	Waist  float64
	Hip    float64
	Result float64
}

type TDEECalculation struct {
	Gender   string
	Age      int
	Weight   float64
	Height   float64
	Activity string
	Goal     string
	Result   float64
}
