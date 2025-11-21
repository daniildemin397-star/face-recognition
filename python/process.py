from flask import Flask, request, jsonify
import os
import cv2
import numpy as np
from face_extractor import FaceExtractor
from cluster_generator import ClusterGenerator

app = Flask(__name__)

# Инициализация моделей при старте
print("🔄 Инициализация моделей...")
face_extractor = FaceExtractor(model_name='buffalo_l', ctx_id=-1)  # CPU
cluster_generator = ClusterGenerator(algorithm='dbscan', eps=0.4, min_samples=1, metric='cosine')
print("✅ Модели загружены")

# Пути должны совпадать с Go сервером!
# Go раздает статику из './uploads', поэтому Python должен туда сохранять
UPLOAD_FOLDER = '../uploads'  # Относительно python/
os.makedirs(UPLOAD_FOLDER, exist_ok=True)

@app.route('/process', methods=['POST'])
def process_images():
    """
    Полная обработка изображений:
    1. Детекция лиц (InsightFace)
    2. Извлечение embeddings
    3. Кластеризация (DBSCAN)
    4. Рисование bbox на изображениях

    Input: multipart/form-data с изображениями
    Output: JSON с кластерами, embeddings и путями к аннотированным фото
    """
    try:
        files = request.files.getlist('images')
        task_id = request.form.get('task_id', 'unknown')

        # Параметры детекции (можно передавать из Go)
        min_size = int(request.form.get('min_size', 30))
        det_thresh = float(request.form.get('det_thresh', 0.5))

        if not files:
            return jsonify({
                'success': False,
                'error': 'Файлы не найдены'
            }), 400

        print(f"\n{'='*70}")
        print(f"📸 Task {task_id}: Получено {len(files)} изображений")
        print(f"{'='*70}")

        # Создаем папку для этой задачи в uploads (где Go раздает статику)
        task_folder = os.path.join(UPLOAD_FOLDER, task_id)
        os.makedirs(task_folder, exist_ok=True)

        # Сохраняем загруженные изображения
        saved_paths = []
        for file in files:
            if file.filename:
                filepath = os.path.join(task_folder, file.filename)
                file.save(filepath)
                saved_paths.append(filepath)
                print(f"  ✓ Сохранен: {file.filename}")

        print(f"\n🔍 Шаг 1: Детекция лиц (min_size={min_size}, det_thresh={det_thresh})")

        # Извлекаем лица из всех изображений
        all_faces = []
        face_counter = 0

        for image_path in saved_paths:
            image_name = os.path.basename(image_path)

            for face_data in face_extractor.extract_faces_from_image_path(
                    image_path,
                    min_size=min_size,
                    det_thresh=det_thresh
            ):
                # Генерируем уникальный ID для каждого лица
                face_id = f"{task_id}_img{len(all_faces)}_face{face_counter}"

                # Сохраняем изображение с bbox В ТУ ЖЕ ПАПКУ что и оригинал
                boxed_image_filename = f"{face_id}_boxed.jpg"
                boxed_image_path = os.path.join(task_folder, boxed_image_filename)
                cv2.imwrite(boxed_image_path, face_data['boxed_image'])

                # Формируем пути относительно uploads/ для Go
                # Go раздает через /uploads/task_id/file.jpg
                original_relative = os.path.join(task_id, os.path.basename(image_path))
                boxed_relative = os.path.join(task_id, boxed_image_filename)

                # Добавляем информацию о лице
                face_info = {
                    'face_id': face_id,
                    'original_image_path': original_relative,  # Относительный путь!
                    'original_image_name': image_name,
                    'boxed_image_path': boxed_relative,        # Относительный путь!
                    'bbox': face_data['bbox'].tolist(),
                    'det_score': face_data['det_score'],
                    'embedding': face_data['embedding']
                }
                all_faces.append(face_info)
                face_counter += 1

            print(f"  • {image_name}: найдено {face_counter} лиц")
            face_counter = 0

        total_faces = len(all_faces)

        if total_faces == 0:
            print("❌ Лица не обнаружены ни на одном изображении")
            return jsonify({
                'success': False,
                'error': 'Лица не обнаружены',
                'total_faces': 0
            })

        print(f"✅ Всего найдено {total_faces} лиц")

        # Подготавливаем данные для кластеризации
        print(f"\n🔄 Шаг 2: Кластеризация {total_faces} лиц")

        embeddings_array = np.array([face['embedding'] for face in all_faces])

        # Кластеризация
        result = cluster_generator.generate_clusters(
            faces=all_faces,
            embeddings=embeddings_array,
            path_key='face_id'
        )

        # Формируем ответ в формате, удобном для Go
        clusters = result['clusters']
        embeddings_dict = result['embeddings']

        # Дополнительная информация о лицах для Go
        faces_metadata = {}
        for face in all_faces:
            faces_metadata[face['face_id']] = {
                'original_image': face['original_image_path'],
                'boxed_image': face['boxed_image_path'],
                'bbox': face['bbox'],
                'confidence': face['det_score']
            }

        print(f"\n📦 Пример путей для проверки:")
        if all_faces:
            sample = all_faces[0]
            print(f"  Original: {sample['original_image_path']}")
            print(f"  Boxed: {sample['boxed_image_path']}")

        unique_persons = len([k for k in clusters.keys() if k != 'noise'])

        print(f"✅ Найдено {unique_persons} уникальных людей")
        for cluster_name, face_ids in clusters.items():
            if cluster_name != 'noise':
                print(f"  • {cluster_name}: {len(face_ids)} лиц")

        if 'noise' in clusters:
            print(f"  ⚠️  noise (outliers): {len(clusters['noise'])} лиц")

        print(f"{'='*70}\n")

        return jsonify({
            'success': True,
            'task_id': task_id,
            'clusters': clusters,
            'embeddings': embeddings_dict,
            'faces_metadata': faces_metadata,
            'total_faces': total_faces,
            'unique_persons': unique_persons
        })

    except Exception as e:
        print(f"\n❌ Ошибка обработки: {str(e)}")
        import traceback
        traceback.print_exc()

        return jsonify({
            'success': False,
            'error': str(e)
        }), 500


@app.route('/health', methods=['GET'])
def health_check():
    """Проверка работоспособности сервиса"""
    return jsonify({
        'status': 'ok',
        'message': 'Python face processor ready',
        'version': '3.0',
        'model': 'InsightFace (buffalo_l)',
        'clustering': 'DBSCAN',
        'features': ['detection', 'embedding', 'clustering', 'bbox_drawing']
    })


@app.route('/compare', methods=['POST'])
def compare_faces():
    """
    Сравнение двух embeddings

    Input: {"embedding1": [...], "embedding2": [...]}
    Output: {"similarity": 0.85, "match": true}
    """
    try:
        data = request.json
        emb1 = np.array(data.get('embedding1'))
        emb2 = np.array(data.get('embedding2'))

        if emb1 is None or emb2 is None:
            return jsonify({'error': 'Требуются оба embedding'}), 400

        # Косинусное сходство
        from sklearn.metrics.pairwise import cosine_similarity
        similarity = float(cosine_similarity([emb1], [emb2])[0][0])

        return jsonify({
            'similarity': similarity,
            'match': similarity > 0.6  # Порог можно настроить
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


if __name__ == '__main__':
    print("\n" + "="*70)
    print("🐍 Face Recognition Processor v3.0 (InsightFace)")
    print("="*70)
    print("Endpoints:")
    print("  POST /process  - Полная обработка (detection + embedding + clustering)")
    print("  POST /compare  - Сравнение двух embeddings")
    print("  GET  /health   - Проверка статуса")
    print("="*70)
    print("Модель: InsightFace buffalo_l (512-dim embeddings)")
    print("Кластеризация: DBSCAN (cosine metric)")
    print("="*70)
    print("Сервер: http://localhost:5000")
    print("="*70 + "\n")

    app.run(host='0.0.0.0', port=5000, debug=True)