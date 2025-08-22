package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetSecret(ctx context.Context, c client.Client, namespace string, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secret)
	if err != nil {
		return secret, err
	}

	return secret, nil
}

func CreateSecret(ctx context.Context, c client.Client, secret *corev1.Secret) error {
	err := c.Create(ctx, secret)
	if err != nil {
		return err
	}

	return nil
}

func DeleteSecret(ctx context.Context, c client.Client, namespace string, name string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	err := c.Delete(ctx, secret)
	if err != nil {
		return err
	}

	return nil
}
