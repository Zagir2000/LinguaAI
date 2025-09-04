#!/usr/bin/env python3
"""
Piper TTS API сервер
Предоставляет HTTP API для синтеза речи через Piper TTS
"""

import os
import subprocess
import tempfile
import json
from pathlib import Path
from typing import Optional

from fastapi import FastAPI, HTTPException, Form
from fastapi.responses import FileResponse
from fastapi.middleware.cors import CORSMiddleware
import uvicorn

app = FastAPI(title="Piper TTS API", version="1.0.0")

# Настройка CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Пути к голосовым моделям
VOICES_DIR = Path("/app/voices")
RUSSIAN_VOICE = {
    "model": VOICES_DIR / "ru_RU-dmitri-medium.onnx",
    "config": VOICES_DIR / "ru_RU-dmitri-medium.onnx.json"
}
ENGLISH_VOICE = {
    "model": VOICES_DIR / "en_US-lessac-medium.onnx", 
    "config": VOICES_DIR / "en_US-lessac-medium.onnx.json"
}

def detect_language(text: str) -> str:
    """Простое определение языка по символам"""
    russian_chars = sum(1 for char in text if 'а' <= char.lower() <= 'я')
    english_chars = sum(1 for char in text if 'a' <= char.lower() <= 'z')
    
    if russian_chars > english_chars:
        return "ru"
    return "en"

def synthesize_speech(text: str, language: Optional[str] = None) -> bytes:
    """Синтезирует речь из текста"""
    if not text.strip():
        raise ValueError("Текст не может быть пустым")
    
    # Определяем язык если не указан
    if not language:
        language = detect_language(text)
    
    # Выбираем голосовую модель
    if language == "ru":
        voice = RUSSIAN_VOICE
    else:
        voice = ENGLISH_VOICE
    
    # Проверяем существование файлов модели
    if not voice["model"].exists() or not voice["config"].exists():
        raise FileNotFoundError(f"Голосовая модель для языка {language} не найдена")
    
    # Создаем временный файл для аудио
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as temp_file:
        temp_path = temp_file.name
    
    try:
        # Выполняем синтез речи
        cmd = [
            "/usr/local/bin/piper",
            "--model", str(voice["model"]),
            "--config", str(voice["config"]),
            "--input_text", text,
            "--output_file", temp_path
        ]
        
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        
        if result.returncode != 0:
            raise RuntimeError(f"Ошибка синтеза речи: {result.stderr}")
        
        # Читаем сгенерированный аудио файл
        with open(temp_path, "rb") as f:
            audio_data = f.read()
        
        return audio_data
    
    finally:
        # Удаляем временный файл
        if os.path.exists(temp_path):
            os.unlink(temp_path)

@app.get("/health")
@app.head("/health")
async def health_check():
    """Проверка здоровья сервиса"""
    return {"status": "healthy", "service": "piper-tts"}

@app.get("/voices")
async def list_voices():
    """Список доступных голосов"""
    voices = []
    
    if RUSSIAN_VOICE["model"].exists():
        voices.append({
            "language": "ru",
            "name": "Dmitri (Russian)",
            "model": str(RUSSIAN_VOICE["model"]),
            "config": str(RUSSIAN_VOICE["config"])
        })
    
    if ENGLISH_VOICE["model"].exists():
        voices.append({
            "language": "en", 
            "name": "Lessac (English)",
            "model": str(ENGLISH_VOICE["model"]),
            "config": str(ENGLISH_VOICE["config"])
        })
    
    return {"voices": voices}

@app.post("/synthesize")
async def synthesize(
    text: str = Form(..., description="Текст для синтеза речи"),
    language: Optional[str] = Form(None, description="Язык (ru/en), если не указан - определяется автоматически")
):
    """Синтез речи из текста"""
    try:
        if len(text) > 1000:
            raise HTTPException(status_code=400, detail="Текст слишком длинный (максимум 1000 символов)")
        
        audio_data = synthesize_speech(text, language)
        
        # Создаем временный файл для ответа
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as temp_file:
            temp_file.write(audio_data)
            temp_path = temp_file.name
        
        return FileResponse(
            temp_path,
            media_type="audio/wav",
            filename="speech.wav",
            headers={"Content-Disposition": "attachment; filename=speech.wav"}
        )
    
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except FileNotFoundError as e:
        raise HTTPException(status_code=500, detail=f"Голосовая модель не найдена: {str(e)}")
    except RuntimeError as e:
        raise HTTPException(status_code=500, detail=f"Ошибка синтеза речи: {str(e)}")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Внутренняя ошибка: {str(e)}")

@app.post("/synthesize-raw")
async def synthesize_raw(
    text: str = Form(..., description="Текст для синтеза речи"),
    language: Optional[str] = Form(None, description="Язык (ru/en), если не указан - определяется автоматически")
):
    """Синтез речи из текста (возвращает raw аудио данные)"""
    try:
        if len(text) > 1000:
            raise HTTPException(status_code=400, detail="Текст слишком длинный (максимум 1000 символов)")
        
        audio_data = synthesize_speech(text, language)
        
        from fastapi.responses import Response
        return Response(
            content=audio_data,
            media_type="audio/wav",
            headers={"Content-Disposition": "attachment; filename=speech.wav"}
        )
    
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except FileNotFoundError as e:
        raise HTTPException(status_code=500, detail=f"Голосовая модель не найдена: {str(e)}")
    except RuntimeError as e:
        raise HTTPException(status_code=500, detail=f"Ошибка синтеза речи: {str(e)}")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Внутренняя ошибка: {str(e)}")

if __name__ == "__main__":
    print("🎵 Запускаем Piper TTS API сервер...")
    print(f"📁 Голосовые модели: {VOICES_DIR}")
    print(f"🇷🇺 Русская модель: {RUSSIAN_VOICE['model'].exists()}")
    print(f"🇺🇸 Английская модель: {ENGLISH_VOICE['model'].exists()}")
    
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=8000,
        log_level="info"
    )
