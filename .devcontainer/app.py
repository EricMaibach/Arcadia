from fastapi import FastAPI, File, UploadFile, HTTPException
from transformers import Blip2Processor, Blip2ForConditionalGeneration
from PIL import Image
import torch
import io
import pillow_heif
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Register HEIF opener with Pillow
pillow_heif.register_heif_opener()

app = FastAPI()

# Global variables for model and processor
model = None
processor = None
device = None

@app.on_event("startup")
async def load_model():
    """Load model and processor on startup"""
    global model, processor, device

    device = "cuda" if torch.cuda.is_available() else "cpu"
    logger.info(f"Using device: {device}")

    logger.info("Loading BLIP-2 processor...")
    processor = Blip2Processor.from_pretrained("Salesforce/blip2-flan-t5-xl")
    logger.info("Processor loaded successfully!")

    logger.info("Loading BLIP-2 model (this may take several minutes)...")
    model = Blip2ForConditionalGeneration.from_pretrained(
        "Salesforce/blip2-flan-t5-xl",
        torch_dtype=torch.float16 if device == "cuda" else torch.float32
    )
    model.to(device)
    logger.info(f"Model loaded successfully on {device}!")
    logger.info("API is ready to accept requests")

@app.post("/caption")
async def caption_image(image: UploadFile = File(...)):
    """Generate a caption for an uploaded image"""
    try:
        logger.info(f"Received caption request for image: {image.filename}")

        # Load image
        contents = await image.read()
        pil_image = Image.open(io.BytesIO(contents)).convert("RGB")
        logger.info(f"Image loaded - size: {pil_image.size}, mode: {pil_image.mode}")

        # Process and generate caption
        prompt = "Question: Describe this image in detail, including objects, people, actions, colors, setting, and atmosphere. Answer:"
        logger.info(f"Processing image with prompt: '{prompt}'")
        inputs = processor(images=pil_image, text=prompt, return_tensors="pt").to(device)

        logger.info("Generating caption...")
        outputs = model.generate(
            **inputs,
            do_sample=False,
            num_beams=5,
            max_length=512,
            min_length=40,
            top_p=0.9,
            repetition_penalty=1.5,
            length_penalty=1.2,
            temperature=1,
        )

        caption = processor.batch_decode(outputs, skip_special_tokens=True)[0].strip()
        logger.info(f"Caption generated successfully: '{caption}'")
        return {"caption": caption}

    except Exception as e:
        logger.error(f"Error processing image: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
