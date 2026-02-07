package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	GiteaID      int64  `gorm:"uniqueIndex" json:"gitea_id"`
	Username     string `gorm:"uniqueIndex" json:"username"`
	Email        string `json:"email"`
	FullName     string `json:"full_name"`
	AvatarURL    string `json:"avatar_url"`
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
	IsAdmin      bool   `gorm:"default:false" json:"is_admin"`

	Courses []Course `gorm:"many2many:course_instructors;" json:"courses,omitempty"`
}

type Course struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name         string `json:"name"`
	Description  string `json:"description"`
	Slug         string `gorm:"uniqueIndex" json:"slug"`
	OrgName      string `json:"org_name"`
	AcademicYear int    `json:"academic_year"`
	InviteCode   string `gorm:"uniqueIndex" json:"invite_code"`

	Instructors []User       `gorm:"many2many:course_instructors;" json:"instructors,omitempty"`
	Assignments []Assignment `json:"assignments,omitempty"`
	Students    []Student    `json:"students,omitempty"`
}

type Assignment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	CourseID uint   `json:"course_id"`
	Course   Course `json:"course,omitempty"`

	Title        string    `json:"title"`
	Description  string    `json:"description"`
	TemplateRepo string    `json:"template_repo"`
	Deadline     time.Time `json:"deadline"`
	MaxPoints    int       `json:"max_points"`
	AcademicYear int       `json:"academic_year"`

	Submissions []Submission `json:"submissions,omitempty"`
}

type Student struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	CourseID uint   `json:"course_id"`
	Course   Course `json:"course,omitempty"`

	GiteaID  int64  `json:"gitea_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`

	Submissions []Submission `json:"submissions,omitempty"`
}

type Submission struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	AssignmentID uint       `json:"assignment_id"`
	Assignment   Assignment `json:"assignment,omitempty"`
	StudentID    uint       `json:"student_id"`
	Student      Student    `json:"student,omitempty"`

	RepoURL     string     `json:"repo_url"`
	Status      string     `json:"status"`
	Score       *int       `json:"score"`
	Feedback    string     `json:"feedback"`
	SubmittedAt *time.Time `json:"submitted_at"`
}

// Review request statuses
const (
	ReviewStatusPending   = "pending"
	ReviewStatusSubmitted = "submitted"
	ReviewStatusReviewed  = "reviewed"
	ReviewStatusCancelled = "cancelled"
)

type ReviewRequest struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	SubmissionID uint       `json:"submission_id"`
	Submission   Submission `json:"submission,omitempty"`

	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	SubmittedAt *time.Time `json:"submitted_at"`
	ReviewedAt  *time.Time `json:"reviewed_at"`
	SheetRowID  int        `json:"sheet_row_id"`
}

type StudentInvite struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	CourseID uint   `json:"course_id"`
	Course   Course `json:"course,omitempty"`

	FullName string  `json:"full_name"`
	Token    *string `gorm:"uniqueIndex" json:"token,omitempty"`
	Used     bool    `gorm:"default:false" json:"used"`
	UsedAt   *time.Time `json:"used_at,omitempty"`

	// Will be filled when student registers
	StudentID *uint   `json:"student_id,omitempty"`
	Student   *Student `json:"student,omitempty"`
}
