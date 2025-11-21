import os
import cv2
from insightface.app import FaceAnalysis


class FaceExtractor:
    def __init__(self, model_name='buffalo_l', ctx_id=-1):
        self.app = FaceAnalysis(name=model_name, providers=['CUDAExecutionProvider', 'CPUExecutionProvider'])
        self.app.prepare(ctx_id=ctx_id, det_size=(640, 640))
        print(f"[FaceExtractor] Модель {model_name} загружена.")

    def extract_faces_from_image_path(self, image_path, min_size=30, det_thresh=0.5):
        img = cv2.imread(image_path)
        if img is None:
            print(f"[ERROR] Не удалось загрузить изображение: {image_path}")
            return

        faces = self.app.get(img)

        for face in faces:
            if face.det_score < det_thresh:
                continue

            bbox = face.bbox.astype(int)
            kps = face.kps

            w, h = bbox[2] - bbox[0], bbox[3] - bbox[1]
            if w < min_size or h < min_size:
                continue

            boxed_img = img.copy()
            cv2.rectangle(boxed_img, (bbox[0], bbox[1]), (bbox[2], bbox[3]), (255, 176, 0), 4)

            yield {
                'bbox': bbox,
                'kps': kps,
                'det_score': float(face.det_score),
                'embedding': face.normed_embedding,
                'boxed_image': boxed_img,
                'original_image_path': image_path
            }

    def extract_faces_from_folder(self, folder_path, min_size=40, det_thresh=0.5):
        image_paths = [
            os.path.join(folder_path, filename)
            for filename in os.listdir(folder_path)
            if filename.lower().endswith(('.jpg', '.jpeg', '.png'))
        ]

        for image_path in image_paths:
            for face in self.extract_faces_from_image_path(image_path, min_size, det_thresh):
                yield face

    def save_boxed_faces_to_folder(self, faces, output_folder):
        os.makedirs(output_folder, exist_ok=True)
        for i, face in enumerate(faces):
            output_path = os.path.join(output_folder, f"boxed_face_{i}.jpg")
            cv2.imwrite(output_path, face['boxed_image'])
        print(f"[FaceExtractor] Сохранено {len(faces)} изображений с обведёнными лицами в {output_folder}")