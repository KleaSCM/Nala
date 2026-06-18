import { mount } from "svelte";
import App from "./App.svelte";
import "./app.css";

const target = document.getElementById("app");

if (target) {
	target.innerHTML = "";
	mount(App, { target });
} else {
	console.error("Nala: #app element not found");
}
