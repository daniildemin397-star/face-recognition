from argparse import ArgumentParser
from face_extractor import FaceExtractor
from cluster_generator import ClusterGenerator
import numpy as np

def main():
    parser = ArgumentParser(description="Extracting faces from Images folder to Faces folder with current min size of face on photo in pixels (min_size) and probability of face (det_thresh)")

    parser.add_argument("--min_size", type=int, help="Minimum size of face to extract", default=30)
    parser.add_argument("--det_thresh", type=float, help="Minimum probability of face to extract", default=0.5)
    parser.add_argument("--device", type=int, help="Device for executing (CPU -1, GPU 0)", default=-1)
    parser.add_argument("--image_folder", "-i", type=str, help="Folder with input image files", default="Images/")
    parser.add_argument("--output_folder", "-o", type=str, help="Folder to save extracted faces", default="Faces/")
    parser.add_argument("--algorithm", "-a", type=str, help="Name of clustering algorithm (dbscan or hdbscan)", default="dbscan")
    parser.add_argument("--eps", "-e", type=float, help="Epsilon parameter for dbscan (radius for each cluster)", default=0.4)
    parser.add_argument("--min_samples", type=int, help="Minimum count of points in each cluster", default=2)
    parser.add_argument("--metric", "-m", type=str, help="Metric for clustering (cosine or euclidean)", default="cosine")

    args = parser.parse_args()

    extractor = FaceExtractor(ctx_id=args.device)
    cluster_gen = ClusterGenerator(algorithm=args.algorithm, eps=args.eps, min_samples=args.min_samples, metric=args.metric)

    faces = list(extractor.extract_faces_from_folder(args.image_folder, min_size=args.min_size, det_thresh=args.det_thresh))

    if not faces:
        print("Лица не найдены!")
        exit()

    embeddings = np.array([f['embedding'] for f in faces])

    result = cluster_gen.generate_clusters(faces, embeddings)

    print(result["celebrity_matches"])

if __name__ == "__main__":
    main()