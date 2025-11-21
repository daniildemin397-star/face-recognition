import numpy as np
from sklearn.cluster import DBSCAN, HDBSCAN
from typing import List, Dict
from sklearn.metrics.pairwise import cosine_similarity
import pickle


class ClusterGenerator:
    def __init__(self, algorithm='dbscan', eps=0.4, min_samples=2, metric='cosine'):
        self.algorithm = algorithm
        self.eps = eps
        self.min_samples = min_samples
        self.metric = metric

    def fit_predict(self, embeddings: np.ndarray) -> np.ndarray:
        if self.algorithm == 'dbscan':
            clustering = DBSCAN(eps=self.eps, min_samples=self.min_samples, metric=self.metric)
        elif self.algorithm == 'hdbscan':
            try:
                clustering = HDBSCAN(min_cluster_size=self.min_samples, metric=self.metric)
            except ImportError:
                raise ImportError("Для HDBSCAN установите: pip install hdbscan")
        else:
            raise ValueError(f"Неизвестный алгоритм: {self.algorithm}")

        labels = clustering.fit_predict(embeddings)
        return labels

    def generate_clusters(
        self,
        faces: List[Dict],
        embeddings: np.ndarray,
        path_key: str = 'original_image_path'
    ) -> Dict:
        labels = self.fit_predict(embeddings)

        clusters = {}
        embeddings_dict = {}

        for i, face in enumerate(faces):
            path = face[path_key]
            label = int(labels[i])
            embedding = embeddings[i].tolist()
            embeddings_dict[path] = embedding

            cluster_name = f"person_{label}" if label != -1 else "noise"
            if cluster_name not in clusters:
                clusters[cluster_name] = []
            clusters[cluster_name].append(path)

        return {
            "success": True,
            "clusters": clusters,
            "embeddings": embeddings_dict
        }