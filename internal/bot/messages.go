package bot

import (
	"fmt"
	"lingua-ai/pkg/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Messages содержит все тексты сообщений бота
type Messages struct{}

// NewMessages создает новый экземпляр сообщений
func NewMessages() *Messages {
	return &Messages{}
}

// Welcome возвращает приветственное сообщение
func (m *Messages) Welcome(firstName, levelText string, xp int) string {
	// Получаем информацию о прогрессе
	xpForNext, _ := models.GetXPForNextLevel(xp)
	progress := models.GetLevelProgress(xp)

	var levelEmoji string
	var progressInfo string

	currentLevel := models.GetLevelByXP(xp)
	switch currentLevel {
	case models.LevelBeginner:
		levelEmoji = "🔵"
		progressInfo = fmt.Sprintf("🎯 До среднего уровня: %d XP (%.1f%%)", xpForNext, progress)
	case models.LevelIntermediate:
		levelEmoji = "🟡"
		progressInfo = fmt.Sprintf("🎯 До продвинутого уровня: %d XP (%.1f%%)", xpForNext, progress)
	case models.LevelAdvanced:
		levelEmoji = "🟢"
		progressInfo = "🏆 Максимальный уровень достигнут!"
	}

	return fmt.Sprintf(`Hi, <b>%s</b>! Let's chat in English 🇬🇧

Я твой <b>AI-преподаватель английского</b>. Давай общаться на английском языке — я буду исправлять ошибки и помогать тебе улучшать язык!

🎯 <b>Как это работает:</b>
• Пиши на английском → получай XP
• Я исправлю ошибки и помогу понять правила
• За правильные ответы — больше очков

📊 <b>Твой уровень:</b> %s %s | ⭐ XP: %d
%s

💡 <b>Система рангов:</b>
🔵 Начинающий | 🟡 Активист | 🟢 Легенда

💰 <b>Система баллов:</b>
+15 XP — правильно
+10 XP — попытка
+3 XP — участие

Try to write something in English 🚀`,
		firstName, levelEmoji, levelText, xp, progressInfo)
}

// Help возвращает справку по командам
func (m *Messages) Help() string {
	return `🇬🇧 <b>Lingua AI — English Chat Assistant</b>

🎯 <b>Главная идея:</b>  
Общайся со мной на английском языке! Я буду исправлять твои ошибки, объяснять правила и помогать улучшать английский в естественном общении.  

💬 <b>Как это работает:</b>  
1️⃣ Пишешь мне на английском — я отвечаю и исправляю ошибки  
2️⃣ Пишешь на русском — я перевожу и предлагаю английский вариант  
3️⃣ Получаешь XP за активность и правильность  

💡 <b>Система баллов:</b>  
• +15 XP — правильное английское сообщение  
• +10 XP — попытка писать на английском  
• +3 XP — участие в диалоге  

📊 <b>Команды:</b>  
• /learning — меню обучения  
• /stats — твоя статистика и прогресс  
• /flashcards — словарные карточки для изучения  
• /clear — очистить историю диалога  
• /premium — управление подпиской  
• /help — справка  

🎤 <b>Голосовые сообщения:</b>  
Говори на английском — я распознаю речь и помогу с произношением!  

📚 <b>Карточки:</b>  
• /flashcards — изучай новые слова с интервальным повторением  
• Алгоритм запоминания подстраивается под твой прогресс  

💎 <b>Премиум-подписка:</b>  
• 🚀 Безлимитные сообщения (бесплатно: 15/день)  
• ⚡ Приоритетная поддержка  
• 🎯 Расширенные упражнения  
• 📈 Персональные рекомендации  

🚀 <i>Just start chatting in English!</i>`
}

// Stats возвращает статистику пользователя
func (m *Messages) Stats(firstName, levelText string, xp, studyStreak int, lastStudyDate string) string {
	xpForNext, _ := models.GetXPForNextLevel(xp)
	progress := models.GetLevelProgress(xp)

	var progressInfo string
	currentLevel := models.GetLevelByXP(xp)

	switch currentLevel {
	case models.LevelBeginner:
		progressInfo = fmt.Sprintf("🎯 До ранга активист: %d XP (%.1f%%)", xpForNext, progress)
	case models.LevelIntermediate:
		progressInfo = fmt.Sprintf("🎯 До продвинутого легенда: %d XP (%.1f%%)", xpForNext, progress)
	case models.LevelAdvanced:
		progressInfo = "🏆 Максимальный ранг достигнут!"
	}

	return fmt.Sprintf(`📊 <b>Твоя статистика</b>

👤 <b>Пользователь:</b> %s  
📈 <b>Уровень английского:</b> %s  
⭐ <b>Опыт:</b> %d XP  
%s  
🔥 <b>Серия дней:</b> %d подряд  
📅 <b>Последнее изучение:</b> %s  

💡 <b>Ранг:</b>  
🔵 Новичок : 0 — 9,999 XP  
🟡 Активист : 10,000 — 19,999 XP  
🟢 Легенда: 20,000+ XP`, firstName, levelText, xp, progressInfo, studyStreak, lastStudyDate)
}

// ChatCleared возвращает сообщение об очистке истории
func (m *Messages) ChatCleared() string {
	return "✅ <b>История диалога очищена!</b>"
}

// UnknownCommand возвращает сообщение о неизвестной команде
func (m *Messages) UnknownCommand() string {
	return "⚠️ Неизвестная команда. Используй <b>/help</b> для справки."
}

// Error возвращает сообщение об ошибке
func (m *Messages) Error(message string) string {
	return fmt.Sprintf("❌ <b>Ошибка:</b> %s\n\nПопробуйте позже или обратитесь к администратору.", message)
}

// GetMainKeyboard возвращает основную клавиатуру
func (m *Messages) GetMainKeyboard() [][]string {
	return [][]string{
		{"📚 Обучение", "📊 Статистика"},
		{"🏆 Рейтинг", "💎 Премиум"},
		{"🔗 Реферальная ссылка", "❓ Помощь"},
		{"🗑 Очистить диалог"},
	}
}

func (m *Messages) GetLearningKeyboard() [][]string {
	return [][]string{
		{"📝 Словарные карточки", "🎓 Тест уровня"},
		{"🔙 Назад в главное меню"},
	}
}

func (m *Messages) LevelTestIntro() string {
	return `🎯 <b>Тест уровня английского</b>

Этот тест поможет определить твой <b>текущий уровень английского языка</b>.  

📋 <b>Что тебя ждёт:</b>  
• 10 вопросов разной сложности  
• Проверка грамматики, лексики и понимания  
• Варианты ответов на каждый вопрос  
• Результат:  
   🔵 Beginner | 🟡 Intermediate | 🟢 Advanced  

⏱ <b>Время:</b> без ограничений — отвечай спокойно  

💡 <i>Совет:</i> можно отменить тест в любой момент  

🚀 Готов начать?  
Нажми <b>«Начать тест»</b>, чтобы приступить!`
}

// LevelTestQuestion возвращает форматированный вопрос теста
func (m *Messages) LevelTestQuestion(questionNum, totalQuestions int, question string, options []string) string {
	text := fmt.Sprintf(`🎯 <b>Вопрос %d из %d</b>

%s

<b>Варианты ответов:</b>`, questionNum, totalQuestions, question)

	for i, option := range options {
		text += fmt.Sprintf("\n%d️⃣ %s", i+1, option)
	}

	text += "\n\n💡 Отправь номер правильного ответа (1–4)"
	text += "\n❌ Чтобы выйти, используй «Отменить тест»"

	return text
}

// GetLevelTestKeyboard возвращает клавиатуру для теста уровня
func (m *Messages) GetLevelTestKeyboard() [][]string {
	return [][]string{
		{"🎯 Начать тест"},
		{"🔙 Назад к меню"},
	}
}

// GetActiveTestKeyboard возвращает клавиатуру для активного теста
func (m *Messages) GetActiveTestKeyboard() [][]string {
	return [][]string{
		{"❌ Отменить тест"},
		{"🔙 Назад к меню"},
	}
}

// GetTestAnswerKeyboard возвращает клавиатуру с вариантами ответов для теста
func (m *Messages) GetTestAnswerKeyboard(options []string) [][]tgbotapi.InlineKeyboardButton {
	var keyboard [][]tgbotapi.InlineKeyboardButton

	// Добавляем кнопки с короткими номерами (варианты показаны в тексте сообщения)
	buttonTexts := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣"}

	for i := range options {
		buttonText := buttonTexts[i]
		if i >= len(buttonTexts) {
			buttonText = fmt.Sprintf("%d", i+1)
		}

		button := tgbotapi.NewInlineKeyboardButtonData(
			buttonText,
			fmt.Sprintf("test_answer_%d", i),
		)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	// Добавляем кнопку отмены
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("❌ Отменить тест", "test_cancel")
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{cancelButton})

	return keyboard
}
