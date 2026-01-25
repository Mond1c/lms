package handlers

import (
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
)

func isInstructor(userID uint, courseID uint) bool {
	var count int64
	database.DB.Table("course_instructors").
		Where("user_id = ? AND course_id = ?", userID, courseID).
		Count(&count)
	return count > 0
}

func isInstructorBySlug(userID uint, slug string) bool {
	var course models.Course
	if err := database.DB.Where("slug = ?", slug).First(&course).Error; err != nil {
		return false
	}
	return isInstructor(userID, course.ID)
}

func isStudentOfCourse(userID uint, courseID uint) bool {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return false
	}

	var count int64
	database.DB.Model(&models.Student{}).
		Where("course_id = ? AND gitea_id = ?", courseID, user.GiteaID).
		Count(&count)
	return count > 0
}
