package main

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TODO: Use generics
type Optional_NamespacedName []types.NamespacedName

func (l Optional_NamespacedName) Contains(v types.NamespacedName) bool {
	for _, lv := range l {
		if lv == v {
			return true
		}
	}
	return false
}

type ImmutableList_NamespacedName []types.NamespacedName

func (l ImmutableList_NamespacedName) Remove(removeV types.NamespacedName) ImmutableList_NamespacedName {
	newList := make([]types.NamespacedName, 0, len(l)-1)
	for _, v := range l {
		if v != removeV {
			newList = append(newList, v)
		}
	}
	return newList
}

func (l ImmutableList_NamespacedName) Add(addV types.NamespacedName) ImmutableList_NamespacedName {
	newList := make([]types.NamespacedName, 0, len(l)+1)

	newList = append(newList, l...)
	newList = append(newList, addV)

	return newList
}

type ImmutableList_Ingress []*networkingv1.Ingress

func (l ImmutableList_Ingress) Remove(removeV *networkingv1.Ingress) ImmutableList_Ingress {
	newList := make([]*networkingv1.Ingress, 0, len(l)-1)
	for _, v := range l {
		if v.Name != removeV.Name || v.Namespace != removeV.Namespace {
			newList = append(newList, v)
		}
	}
	return newList
}

func (l ImmutableList_Ingress) Add(addV *networkingv1.Ingress) ImmutableList_Ingress {
	newList := make([]*networkingv1.Ingress, 0, len(l)+1)

	newList = append(newList, l...)
	newList = append(newList, addV)

	return newList
}

func (l ImmutableList_Ingress) Replace(replaceV *networkingv1.Ingress) ImmutableList_Ingress {
	newList := make([]*networkingv1.Ingress, 0, len(l)-1)
	for _, v := range l {
		if v.Name != replaceV.Name || v.Namespace != replaceV.Namespace {
			newList = append(newList, v)
		} else {
			newList = append(newList, replaceV)
		}
	}
	return newList
}

type ImmutableList_EndpointSlice []*discoveryv1.EndpointSlice

func (l ImmutableList_EndpointSlice) Remove(removeV *discoveryv1.EndpointSlice) ImmutableList_EndpointSlice {
	newList := make([]*discoveryv1.EndpointSlice, 0, len(l)-1)
	for _, v := range l {
		if v.Name != removeV.Name || v.Namespace != removeV.Namespace {
			newList = append(newList, v)
		}
	}
	return newList
}

func (l ImmutableList_EndpointSlice) Add(addV *discoveryv1.EndpointSlice) ImmutableList_EndpointSlice {
	newList := make([]*discoveryv1.EndpointSlice, 0, len(l)+1)

	newList = append(newList, l...)
	newList = append(newList, addV)

	return newList
}

func (l ImmutableList_EndpointSlice) Replace(replaceV *discoveryv1.EndpointSlice) ImmutableList_EndpointSlice {
	newList := make([]*discoveryv1.EndpointSlice, 0, len(l)-1)
	for _, v := range l {
		if v.Name != replaceV.Name || v.Namespace != replaceV.Namespace {
			newList = append(newList, v)
		} else {
			newList = append(newList, replaceV)
		}
	}
	return newList
}
